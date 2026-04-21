# AGENTS.md - Local Overlay Tracking

This repository is a moving fork. Treat everything marked with `LOCAL FIX START` / `LOCAL FIX END` as an explicit local overlay that must be reviewed during every upstream sync.

## Source Of Local Overlays

- Base remote: `origin -> https://github.com/dmzz/goclaw`
- Imported patch source: `https://github.com/alsenis/goclaw/tree/upgrade/v3.9.1`
- Imported commits currently carried locally:
  - `f5563388` `fix(telegram): deliver existing files in current chat`
  - `08a834f7` `patch: consolidate telegram and tool-loop fixes`

## Recommended Branch Layout

For a fast-moving upstream, do not keep local patches only on the long-lived `dev` branch.

Recommended layout:

- `upstream` remote:
  - `https://github.com/nextlevelbuilder/goclaw`
- `base/vX.Y.Z`:
  - exact local branch pointing to the clean upstream release/tag
- `overlay/vX.Y.Z-dmzz`:
  - local patch branch created from `base/vX.Y.Z`
  - carries only dmzz-specific commits
- `origin/dev`:
  - optional integration branch in your fork
  - update it only from a validated `overlay/*` branch

Practical workflow:

1. Fetch upstream tags.
2. Create `base/vX.Y.Z` from the upstream tag.
3. Create `overlay/vX.Y.Z-dmzz` from that base branch.
4. Cherry-pick or rebase the local overlay commits onto the new overlay branch.
5. Run validation.
6. Only then fast-forward or merge into the branch you actually deploy from.

This keeps the diff against upstream small, makes rebases predictable, and lets you compare:

- `base/vX.Y.Z..overlay/vX.Y.Z-dmzz`
- `overlay/vX.Y.Z-dmzz..overlay/vX.Y.(Z+1)-dmzz`

## Local Overlay Areas

### 1. Telegram message delivery fixes

- Goal: allow sending already-existing files into the current Telegram chat and recover from invalid Telegram `target` placeholders like `current chat`.
- Inline markers live in:
  - `internal/tools/message.go`
  - `internal/tools/message_test.go`

### 2. Telegram multi-bot routing fixes

- Goal: improve multi-bot behavior in Telegram groups.
- Behavior covered:
  - prompt augmentation so exact `@username` handles survive handoff
  - Unicode-safe mention/entity parsing
  - `allow_bot_messages`
  - bot-to-bot loop guard in `yield` mode
  - group/topic config propagation for the above
- Inline markers live in:
  - `cmd/gateway_consumer_normal.go`
  - `cmd/gateway_consumer_prompt_helpers.go`
  - `cmd/gateway_consumer_prompt_helpers_test.go`
  - `internal/channels/telegram/channel.go`
  - `internal/channels/telegram/channel_parse_test.go`
  - `internal/channels/telegram/factory.go`
  - `internal/channels/telegram/handlers.go`
  - `internal/channels/telegram/handlers_utils.go`
  - `internal/channels/telegram/handlers_utils_test.go`
  - `internal/channels/telegram/mentions.go`
  - `internal/channels/telegram/mentions_test.go`
  - `internal/channels/telegram/topic_config.go`
  - `internal/channels/telegram/topic_config_test.go`
  - `internal/config/config_channels.go`
  - `ui/web/src/pages/channels/channel-detail/channel-advanced-dialog.tsx`
  - `ui/web/src/pages/channels/channel-detail/channel-general-tab.tsx`
  - `ui/web/src/pages/channels/channel-schemas.ts`
  - `ui/web/src/pages/channels/telegram-group-fields.tsx`
  - `ui/web/src/pages/config/sections/channels-section.tsx`

### 3. Tool-loop / Responses API fixes

- Goal: stabilize tool-loop behavior around compaction and OpenAI Responses API edge cases.
- Behavior covered:
  - preserve pending tool calls across compaction
  - retry empty-args `web_search` / `web_fetch` calls as truncation
  - clear retry batches out of `LastResponse` so malformed tool-calls do not leak into `ToolStage`
  - emit a user-facing truncation/tool-call error instead of falling through to final `...`
  - drop orphaned `function_call_output` items for Codex/Responses API
  - tolerate cumulative/overlapping streamed `function.arguments` chunks from OpenAI chat-completions routes
  - recover the last valid JSON object when OpenAI-compatible routes stream concatenated or partially repeated tool-call argument snapshots
  - strip persisted internal retry noise (`[System]` truncation hints, empty assistant stubs, fallback `...`) out of session history before it can poison later prompts
- Inline markers live in:
  - `internal/agent/loop_history_sanitize.go`
  - `internal/agent/loop_history_test.go`
  - `internal/pipeline/prune_stage.go`
  - `internal/pipeline/pipeline_test.go`
  - `internal/pipeline/stages_test.go`
  - `internal/pipeline/think_stage.go`
  - `internal/pipeline/think_stage_trunc_empty_args_test.go`
  - `internal/providers/codex_build.go`
  - `internal/providers/openai_chat.go`
  - `internal/providers/openai_http.go`
  - `internal/providers/openai_truncation_test.go`
  - `internal/providers/codex_test.go`

### 4. Team lead routing bias

- Goal: make lead agents actually use the team workflow when the user explicitly asks for team execution, and make the `search -> create` order clearer to weaker models.
- Behavior covered:
  - lead TEAM.md example now shows `team_tasks(search)` before `team_tasks(create)`
  - explicit user requests to use the team/teammates must go through `team_tasks`
  - research/report/summarize workflows are biased toward delegation instead of solo execution
- Inline markers live in:
  - `internal/agent/resolver_helpers.go`
  - `internal/agent/loop_utils_test.go`

### 5. Skill frontmatter normalization fixes

- Goal: let skill creation/publication recover when models send `SKILL.md` frontmatter with escaped newlines like `---\nname: ...` instead of real line breaks.
- Behavior covered:
  - normalize escaped `\n`, `\r\n`, and `\t` sequences before parsing frontmatter
  - reuse the normalized content in both `skill_manage` and `publish_skill`
  - persist normalized `SKILL.md` content when a copied skill directory arrived with escaped frontmatter
  - keep regression coverage for escaped-frontmatter inputs
- Inline markers live in:
  - `internal/skills/helpers.go`
  - `internal/skills/helpers_test.go`
  - `internal/tools/publish_skill.go`
  - `internal/tools/skill_manage.go`

### 6. Sandbox container recovery fixes

- Goal: let sandboxed file/exec tools survive Docker daemon or Docker Desktop restarts without getting stuck on stale container names.
- Behavior covered:
  - detect `docker run ... container name is already in use` for `goclaw-sbx-*`
  - inspect the conflicting container and verify it is our sandbox via label `goclaw.sandbox=true`
  - reuse the existing sandbox when it is still running
  - remove the stale sandbox and retry when it is `exited`/dead after daemon restart
  - keep regression coverage for inspect parsing and name-conflict detection
- Inline markers live in:
  - `internal/sandbox/docker.go`
  - `internal/sandbox/docker_test.go`

## Companion Files Without Inline Markers

These belong to the same imported overlay set but cannot be cleanly tracked with inline code comments:

- `docs/05-channels-messaging.md`
- `ui/web/src/i18n/locales/en/channels.json`
- `ui/web/src/i18n/locales/en/config.json`
- `ui/web/src/i18n/locales/vi/channels.json`
- `ui/web/src/i18n/locales/vi/config.json`
- `ui/web/src/i18n/locales/zh/channels.json`
- `ui/web/src/i18n/locales/zh/config.json`

## Update Workflow

When syncing from upstream/community branches:

1. Update/fetch upstream refs as usual.
2. Rebase or merge.
3. Run `rg -n "LOCAL FIX START|LOCAL FIX END" .` and inspect every surviving block.
4. Compare each marked block with upstream code:
   - keep it if upstream still lacks the behavior
   - shrink or remove it if upstream gained the same fix
   - update related tests at the same time
5. Also re-check the companion files listed above, because they are not inline-marked.

### Sandbox Containers During Core Updates

- `goclaw-sbx-*` containers are ephemeral sandbox runtimes created by GoClaw from `goclaw-sandbox:bookworm-slim`; they are not long-lived deploy services.
- When updating `goclaw` core or restarting Docker Desktop / the remote daemon, do not preserve old `goclaw-sbx-*` containers as state. They may survive as `Exited` containers and then block new tool runs by name conflict.
- Before or immediately after restarting the main `goclaw` service, prune stale exited sandbox containers:
  - `DOCKER_API_VERSION=1.47 docker -H tcp://192.168.11.1:2375 ps -aq --filter label=goclaw.sandbox=true --filter status=exited | xargs -r DOCKER_API_VERSION=1.47 docker -H tcp://192.168.11.1:2375 rm -f`
- Verify sandbox tail state with:
  - `DOCKER_API_VERSION=1.47 docker -H tcp://192.168.11.1:2375 ps -a --filter label=goclaw.sandbox=true`
- Rebuild `goclaw-sandbox:bookworm-slim` only when `Dockerfile.sandbox.remote` or the sandbox toolchain changed. A normal GoClaw core update does not require rebuilding every sandbox container.
- If file tools (`read_file`, `write_file`, `list_files`, `exec`) suddenly start failing with `container name is already in use`, treat that first as a stale sandbox cleanup problem, not as an agent logic regression.

## Validation After Any Upstream Sync

- `go test ./cmd/... ./internal/channels/telegram ./internal/config ./internal/pipeline ./internal/providers ./internal/tools/...`
- `pnpm -C ui/web install --frozen-lockfile --force`
- `pnpm -C ui/web build`

## Local Toolchain Assumptions In This Workspace

- `go 1.26.0`
- `node 22.22.0`
- `pnpm 10.30.1`
