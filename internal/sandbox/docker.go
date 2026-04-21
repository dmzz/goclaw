package sandbox

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"maps"
	"os/exec"
	"strings"
	"sync"
	"time"
)

// CheckDockerAvailable verifies that the Docker CLI and daemon are accessible.
// Returns nil if Docker is ready, or an error describing the failure.
func CheckDockerAvailable(ctx context.Context) error {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	out, err := exec.CommandContext(ctx, "docker", "info", "--format", "{{.ServerVersion}}").CombinedOutput()
	if err != nil {
		return fmt.Errorf("docker not available: %w (output: %s)", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// DockerSandbox is a sandbox backed by a Docker container.
type DockerSandbox struct {
	containerID string
	config      Config
	workspace   string
	createdAt   time.Time
	lastUsed    time.Time
	mu          sync.Mutex // protects lastUsed
}

// LOCAL FIX START: inspect stale sandbox container on name conflict
type dockerInspectInfo struct {
	containerID string
	status      string
	isSandbox   bool
}

// LOCAL FIX END: inspect stale sandbox container on name conflict

// newDockerSandbox creates and starts a Docker container for sandboxed execution.
// Matching TS buildSandboxCreateArgs() + createSandboxContainer().
func newDockerSandbox(ctx context.Context, name string, cfg Config, workspace string) (*DockerSandbox, error) {
	args := []string{
		"run", "-d",
		"--name", name,
		"--label", "goclaw.sandbox=true",
	}

	// Security hardening (matching TS buildSandboxCreateArgs)
	if cfg.ReadOnlyRoot {
		args = append(args, "--read-only")
	}
	for _, t := range cfg.Tmpfs {
		if !strings.Contains(t, ":") {
			// Always add security flags; optionally add size limit
			opts := "noexec,nosuid,nodev"
			if cfg.TmpfsSizeMB > 0 {
				opts = fmt.Sprintf("size=%dm,%s", cfg.TmpfsSizeMB, opts)
			}
			t = fmt.Sprintf("%s:%s", t, opts)
		} else if !strings.Contains(t, "noexec") {
			// User-specified options but missing noexec — append security flags
			t += ",noexec,nosuid,nodev"
		}
		args = append(args, "--tmpfs", t)
	}
	for _, cap := range cfg.CapDrop {
		args = append(args, "--cap-drop", cap)
	}
	args = append(args, "--security-opt", "no-new-privileges")

	// Non-root user (reduces attack surface)
	if cfg.User != "" {
		args = append(args, "--user", cfg.User)
	}

	// Resource limits
	if cfg.MemoryMB > 0 {
		args = append(args, "--memory", fmt.Sprintf("%dm", cfg.MemoryMB))
	}
	if cfg.CPUs > 0 {
		args = append(args, "--cpus", fmt.Sprintf("%.1f", cfg.CPUs))
	}
	if cfg.PidsLimit > 0 {
		args = append(args, "--pids-limit", fmt.Sprintf("%d", cfg.PidsLimit))
	}

	// Network
	if !cfg.NetworkEnabled {
		args = append(args, "--network", "none")
	}

	// Workspace mount — resolve host path for DooD (Docker-out-of-Docker) setups.
	containerWorkdir := cfg.ContainerWorkdir()
	if workspace != "" && cfg.WorkspaceAccess != AccessNone {
		mountOpt := "rw"
		if cfg.WorkspaceAccess == AccessRO {
			mountOpt = "ro"
		}
		hostPath := resolveHostWorkspacePath(ctx, workspace)
		args = append(args, "-v", fmt.Sprintf("%s:%s:%s", hostPath, containerWorkdir, mountOpt))
	}
	args = append(args, "-w", containerWorkdir)

	// Environment variables
	for k, v := range cfg.Env {
		args = append(args, "-e", k+"="+v)
	}

	// Image + keep-alive command
	args = append(args, cfg.Image, "sleep", "infinity")

	slog.Debug("creating sandbox container", "name", name, "args", args)

	cmd := exec.CommandContext(ctx, "docker", args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	// LOCAL FIX START: recover from stale goclaw-sbx-* container name conflicts
	runErr := cmd.Run()
	if runErr != nil {
		recovered, retry, recoverErr := recoverSandboxNameConflict(ctx, name, cfg, workspace, stderr.String())
		if recoverErr != nil && isDockerNameConflict(stderr.String()) {
			slog.Warn("sandbox name conflict recovery failed", "name", name, "error", recoverErr)
		}
		if recovered != nil {
			return recovered, nil
		}
		if retry {
			stdout.Reset()
			stderr.Reset()
			cmd = exec.CommandContext(ctx, "docker", args...)
			cmd.Stdout = &stdout
			cmd.Stderr = &stderr
			runErr = cmd.Run()
		}
	}
	if runErr != nil {
		return nil, fmt.Errorf("docker run failed: %w\nstderr: %s", runErr, stderr.String())
	}

	containerID := shortenContainerID(strings.TrimSpace(stdout.String()))
	// LOCAL FIX END: recover from stale goclaw-sbx-* container name conflicts

	slog.Info("sandbox container created", "id", containerID, "name", name, "image", cfg.Image)

	// Run optional setup command (matching TS setupCommand)
	if cfg.SetupCommand != "" {
		setupCmd := exec.CommandContext(ctx, "docker", "exec", "-i", containerID, "sh", "-lc", cfg.SetupCommand)
		if out, err := setupCmd.CombinedOutput(); err != nil {
			slog.Warn("sandbox setup command failed", "id", containerID, "error", err, "output", string(out))
		} else {
			slog.Info("sandbox setup command completed", "id", containerID)
		}
	}

	now := time.Now()
	return &DockerSandbox{
		containerID: containerID,
		config:      cfg,
		workspace:   workspace,
		createdAt:   now,
		lastUsed:    now,
	}, nil
}

// LOCAL FIX START: helper flow for sandbox container recovery after daemon restarts
func recoverSandboxNameConflict(ctx context.Context, name string, cfg Config, workspace, stderr string) (*DockerSandbox, bool, error) {
	if !isDockerNameConflict(stderr) {
		return nil, false, nil
	}

	info, err := inspectDockerContainer(ctx, name)
	if err != nil {
		return nil, false, err
	}
	if !info.isSandbox {
		return nil, false, fmt.Errorf("conflicting container %q is not labeled goclaw.sandbox=true", name)
	}

	now := time.Now()
	switch info.status {
	case "running", "restarting":
		slog.Warn("sandbox name conflict: adopting existing container", "name", name, "id", info.containerID, "status", info.status)
		return &DockerSandbox{
			containerID: shortenContainerID(info.containerID),
			config:      cfg,
			workspace:   workspace,
			createdAt:   now,
			lastUsed:    now,
		}, false, nil
	default:
		if err := removeDockerContainer(ctx, name); err != nil {
			return nil, false, err
		}
		slog.Info("removed stale sandbox container after name conflict", "name", name, "id", info.containerID, "status", info.status)
		return nil, true, nil
	}
}

func inspectDockerContainer(ctx context.Context, name string) (dockerInspectInfo, error) {
	out, err := exec.CommandContext(ctx, "docker", "inspect", "--format", "{{.Id}}|{{.State.Status}}|{{index .Config.Labels \"goclaw.sandbox\"}}", name).CombinedOutput()
	if err != nil {
		return dockerInspectInfo{}, fmt.Errorf("inspect conflicting container %q: %w (output: %s)", name, err, strings.TrimSpace(string(out)))
	}
	return parseDockerInspectInfo(strings.TrimSpace(string(out)))
}

func parseDockerInspectInfo(line string) (dockerInspectInfo, error) {
	parts := strings.Split(line, "|")
	if len(parts) != 3 {
		return dockerInspectInfo{}, fmt.Errorf("unexpected docker inspect output %q", line)
	}
	id := strings.TrimSpace(parts[0])
	status := strings.TrimSpace(parts[1])
	label := strings.TrimSpace(parts[2])
	if id == "" || status == "" {
		return dockerInspectInfo{}, fmt.Errorf("incomplete docker inspect output %q", line)
	}
	return dockerInspectInfo{
		containerID: id,
		status:      status,
		isSandbox:   label == "true",
	}, nil
}

func removeDockerContainer(ctx context.Context, name string) error {
	out, err := exec.CommandContext(ctx, "docker", "rm", "-f", name).CombinedOutput()
	if err != nil {
		return fmt.Errorf("remove stale container %q: %w (output: %s)", name, err, strings.TrimSpace(string(out)))
	}
	return nil
}

func isDockerNameConflict(stderr string) bool {
	return strings.Contains(stderr, "container name") && strings.Contains(stderr, "already in use")
}

func shortenContainerID(id string) string {
	if len(id) > 12 {
		return id[:12]
	}
	return id
}

// LOCAL FIX END: helper flow for sandbox container recovery after daemon restarts

// Exec runs a command inside the container.
// Optional ExecOption (e.g. WithEnv) injects per-call env vars via docker exec -e.
func (s *DockerSandbox) Exec(ctx context.Context, command []string, workDir string, opts ...ExecOption) (*ExecResult, error) {
	s.mu.Lock()
	s.lastUsed = time.Now()
	s.mu.Unlock()

	timeout := time.Duration(s.config.TimeoutSec) * time.Second
	if timeout <= 0 {
		timeout = 5 * time.Minute
	}

	execCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	o := ApplyExecOpts(opts)

	args := []string{"exec"}
	// Inject env vars as -e flags before containerID (credentialed exec)
	for k, v := range o.Env {
		args = append(args, "-e", k+"="+v)
	}
	if workDir != "" {
		args = append(args, "-w", workDir)
	}
	args = append(args, s.containerID)
	args = append(args, command...)

	cmd := exec.CommandContext(execCtx, "docker", args...)

	// Limit output capture to prevent OOM from large command output
	maxOut := s.config.MaxOutputBytes
	if maxOut <= 0 {
		maxOut = 1 << 20 // 1MB default
	}
	stdout := &limitedBuffer{max: maxOut}
	stderr := &limitedBuffer{max: maxOut}
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err := cmd.Run()
	exitCode := 0
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			exitCode = exitErr.ExitCode()
		} else {
			return nil, fmt.Errorf("docker exec: %w", err)
		}
	}

	result := &ExecResult{
		ExitCode: exitCode,
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
	}
	if stdout.truncated {
		result.Stdout += "\n...[output truncated]"
	}
	if stderr.truncated {
		result.Stderr += "\n...[output truncated]"
	}
	return result, nil
}

// Destroy removes the container.
func (s *DockerSandbox) Destroy(ctx context.Context) error {
	cmd := exec.CommandContext(ctx, "docker", "rm", "-f", s.containerID)
	if err := cmd.Run(); err != nil {
		slog.Warn("failed to remove sandbox container", "id", s.containerID, "error", err)
		return err
	}
	slog.Info("sandbox container destroyed", "id", s.containerID)
	return nil
}

// ID returns the container ID.
func (s *DockerSandbox) ID() string { return s.containerID }

// DockerManager manages Docker sandbox containers based on scope.
type DockerManager struct {
	config    Config
	sandboxes map[string]*DockerSandbox
	mu        sync.RWMutex
	stopCh    chan struct{} // signals pruning goroutine to stop
}

// NewDockerManager creates a manager for Docker sandboxes.
// Automatically starts background pruning if configured.
func NewDockerManager(cfg Config) *DockerManager {
	m := &DockerManager{
		config:    cfg,
		sandboxes: make(map[string]*DockerSandbox),
		stopCh:    make(chan struct{}),
	}
	m.startPruning()
	return m
}

// Get returns an existing sandbox or creates a new one for the given key.
// If cfgOverride is non-nil, it is used for new containers instead of the global config.
func (m *DockerManager) Get(ctx context.Context, key string, workspace string, cfgOverride *Config) (Sandbox, error) {
	cfg := m.config
	if cfgOverride != nil {
		cfg = *cfgOverride
	}
	if cfg.Mode == ModeOff {
		return nil, ErrSandboxDisabled
	}

	m.mu.RLock()
	if sb, ok := m.sandboxes[key]; ok {
		m.mu.RUnlock()
		return sb, nil
	}
	m.mu.RUnlock()

	m.mu.Lock()
	defer m.mu.Unlock()

	// Double-check
	if sb, ok := m.sandboxes[key]; ok {
		return sb, nil
	}

	prefix := cfg.ContainerPrefix
	if prefix == "" {
		prefix = "goclaw-sbx-"
	}
	name := prefix + sanitizeKey(key)
	sb, err := newDockerSandbox(ctx, name, cfg, workspace)
	if err != nil {
		return nil, err
	}

	m.sandboxes[key] = sb
	return sb, nil
}

// Release destroys a sandbox by key.
func (m *DockerManager) Release(ctx context.Context, key string) error {
	m.mu.Lock()
	sb, ok := m.sandboxes[key]
	if ok {
		delete(m.sandboxes, key)
	}
	m.mu.Unlock()

	if ok {
		return sb.Destroy(ctx)
	}
	return nil
}

// ReleaseAll destroys all active sandboxes.
func (m *DockerManager) ReleaseAll(ctx context.Context) error {
	m.mu.Lock()
	sbs := make(map[string]*DockerSandbox, len(m.sandboxes))
	maps.Copy(sbs, m.sandboxes)
	m.sandboxes = make(map[string]*DockerSandbox)
	m.mu.Unlock()

	for key, sb := range sbs {
		if err := sb.Destroy(ctx); err != nil {
			slog.Warn("failed to release sandbox", "key", key, "error", err)
		}
	}
	return nil
}

// Stats returns information about active sandboxes.
func (m *DockerManager) Stats() map[string]any {
	m.mu.RLock()
	defer m.mu.RUnlock()

	containers := make(map[string]string, len(m.sandboxes))
	for key, sb := range m.sandboxes {
		containers[key] = sb.containerID
	}

	return map[string]any{
		"mode":       m.config.Mode,
		"image":      m.config.Image,
		"active":     len(m.sandboxes),
		"containers": containers,
	}
}

// Stop signals the pruning goroutine to stop.
// Called during shutdown before ReleaseAll.
func (m *DockerManager) Stop() {
	select {
	case <-m.stopCh:
		// already closed
	default:
		close(m.stopCh)
	}
}

// startPruning launches a background goroutine that periodically prunes idle/old containers.
// Matching TS maybePruneSandboxes().
func (m *DockerManager) startPruning() {
	interval := time.Duration(m.config.PruneIntervalMin) * time.Minute
	if interval <= 0 {
		interval = 5 * time.Minute
	}

	go func() {
		ticker := time.NewTicker(interval)
		defer ticker.Stop()

		for {
			select {
			case <-m.stopCh:
				return
			case <-ticker.C:
				m.Prune(context.Background())
			}
		}
	}()

	slog.Debug("sandbox pruning started", "interval", interval)
}

// Prune removes containers that are idle too long or exceed max age.
// Matching TS SandboxPruneSettings (idleHours, maxAgeDays).
func (m *DockerManager) Prune(ctx context.Context) {
	idleHours := m.config.IdleHours
	if idleHours <= 0 {
		idleHours = 24
	}
	maxAgeDays := m.config.MaxAgeDays
	if maxAgeDays <= 0 {
		maxAgeDays = 7
	}

	now := time.Now()
	idleThreshold := now.Add(-time.Duration(idleHours) * time.Hour)
	ageThreshold := now.Add(-time.Duration(maxAgeDays) * 24 * time.Hour)

	// Collect keys to prune
	m.mu.RLock()
	var toRemove []string
	for key, sb := range m.sandboxes {
		sb.mu.Lock()
		lastUsed := sb.lastUsed
		created := sb.createdAt
		sb.mu.Unlock()

		if lastUsed.Before(idleThreshold) || created.Before(ageThreshold) {
			toRemove = append(toRemove, key)
		}
	}
	m.mu.RUnlock()

	if len(toRemove) == 0 {
		return
	}

	// Remove them
	for _, key := range toRemove {
		m.mu.Lock()
		sb, ok := m.sandboxes[key]
		if ok {
			delete(m.sandboxes, key)
		}
		m.mu.Unlock()

		if ok {
			if err := sb.Destroy(ctx); err != nil {
				slog.Warn("prune: failed to destroy sandbox", "key", key, "error", err)
			} else {
				slog.Info("pruned idle sandbox container", "key", key, "container", sb.containerID)
			}
		}
	}

	slog.Info("sandbox prune completed", "removed", len(toRemove))
}

// sanitizeKey makes a key safe for Docker container names.
func sanitizeKey(key string) string {
	safe := strings.NewReplacer(
		":", "-",
		"/", "-",
		" ", "-",
		".", "-",
	).Replace(key)

	if len(safe) > 50 {
		safe = safe[:50]
	}
	return safe
}

// limitedBuffer is a bytes.Buffer that stops accepting writes after max bytes.
// Prevents OOM when commands produce large output.
type limitedBuffer struct {
	buf       bytes.Buffer
	max       int
	truncated bool
}

func (lb *limitedBuffer) Write(p []byte) (int, error) {
	if lb.truncated {
		return len(p), nil // discard silently
	}
	remaining := lb.max - lb.buf.Len()
	if remaining <= 0 {
		lb.truncated = true
		return len(p), nil
	}
	if len(p) > remaining {
		lb.buf.Write(p[:remaining])
		lb.truncated = true
		return len(p), nil
	}
	return lb.buf.Write(p)
}

func (lb *limitedBuffer) String() string {
	return lb.buf.String()
}
