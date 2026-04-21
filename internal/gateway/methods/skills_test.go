package methods

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/store"
	"github.com/nextlevelbuilder/goclaw/pkg/protocol"
)

// ---- stub SkillStore ----

type stubSkillStore struct {
	skills    []store.SkillInfo
	content   map[string]string
	getCalls  []string
	loadCalls []string
	version   int64
}

func newStubSkillStore(skills []store.SkillInfo) *stubSkillStore {
	content := make(map[string]string, len(skills))
	for _, s := range skills {
		content[s.Slug] = "# " + s.Name + " skill content"
	}
	return &stubSkillStore{skills: skills, content: content, version: 1}
}

func (s *stubSkillStore) ListSkills(_ context.Context) []store.SkillInfo { return s.skills }

func (s *stubSkillStore) GetSkill(_ context.Context, name string) (*store.SkillInfo, bool) {
	s.getCalls = append(s.getCalls, name)
	for i := range s.skills {
		if s.skills[i].Slug == name {
			return &s.skills[i], true
		}
	}
	return nil, false
}

func (s *stubSkillStore) LoadSkill(_ context.Context, name string) (string, bool) {
	s.loadCalls = append(s.loadCalls, name)
	c, ok := s.content[name]
	return c, ok
}

func (s *stubSkillStore) LoadForContext(_ context.Context, _ []string) string { return "" }
func (s *stubSkillStore) BuildSummary(_ context.Context, _ []string) string   { return "" }
func (s *stubSkillStore) FilterSkills(_ context.Context, _ []string) []store.SkillInfo {
	return nil
}
func (s *stubSkillStore) Version() int64 { return s.version }
func (s *stubSkillStore) BumpVersion()   { s.version++ }
func (s *stubSkillStore) Dirs() []string { return nil }

// ---- helpers ----

func buildSkillMethods(t *testing.T, skills []store.SkillInfo) *SkillsMethods {
	t.Helper()
	return NewSkillsMethods(newStubSkillStore(skills), nil)
}

func skillReqFrame(t *testing.T, method string, params map[string]any) *protocol.RequestFrame {
	t.Helper()
	raw, err := json.Marshal(params)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	return &protocol.RequestFrame{
		Type:   protocol.FrameTypeRequest,
		ID:     "sk-req-1",
		Method: method,
		Params: raw,
	}
}

// ---- Tests: handleList ----

func TestSkillsList_EmptyStore_ReturnsEmptyList(t *testing.T) {
	m := buildSkillMethods(t, nil)
	client := nullClient()
	req := skillReqFrame(t, protocol.MethodSkillsList, map[string]any{})
	m.handleList(context.Background(), client, req)
	// No panic = success
}

func TestSkillsList_WithSkills_ReturnsAll(t *testing.T) {
	skills := []store.SkillInfo{
		{Name: "skill-a", Slug: "skill-a", Enabled: true, IsSystem: true},
		{Name: "skill-b", Slug: "skill-b", Enabled: false},
	}
	m := buildSkillMethods(t, skills)
	client := nullClient()
	req := skillReqFrame(t, protocol.MethodSkillsList, map[string]any{})
	m.handleList(context.Background(), client, req)
	// No panic = success
}

// ---- Tests: handleGet ----

func TestSkillsGet_MissingName_ReturnsInvalidRequest(t *testing.T) {
	m := buildSkillMethods(t, nil)
	client := nullClient()
	// Empty params — name is required
	req := skillReqFrame(t, protocol.MethodSkillsGet, map[string]any{})
	m.handleGet(context.Background(), client, req)
	// No panic = invalid-request path hit
}

func TestSkillsGet_SkillNotFound_ReturnsNotFound(t *testing.T) {
	m := buildSkillMethods(t, nil) // empty store
	client := nullClient()
	req := skillReqFrame(t, protocol.MethodSkillsGet, map[string]any{"name": "missing-skill"})
	m.handleGet(context.Background(), client, req)
	// No panic = not-found path hit
}

func TestSkillsGet_ExistingSkill_ReturnsContent(t *testing.T) {
	skills := []store.SkillInfo{
		{Name: "my-skill", Slug: "my-skill", Description: "Test skill", Enabled: true},
	}
	m := buildSkillMethods(t, skills)
	client := nullClient()
	req := skillReqFrame(t, protocol.MethodSkillsGet, map[string]any{"name": "my-skill"})
	m.handleGet(context.Background(), client, req)
	// No panic = found path hit
}

func TestSkillsGet_DisplayNameFallback_UsesSlug(t *testing.T) {
	// LOCAL FIX START: ensure skills.get resolves display names through the listed slug
	store := newStubSkillStore([]store.SkillInfo{
		{Name: "Statham Meme Quotes", Slug: "statham-meme-quotes", Description: "Test skill", Enabled: true},
	})
	m := NewSkillsMethods(store, nil)
	client := nullClient()
	req := skillReqFrame(t, protocol.MethodSkillsGet, map[string]any{"name": "Statham Meme Quotes"})

	m.handleGet(context.Background(), client, req)

	if len(store.getCalls) != 2 {
		t.Fatalf("expected 2 get calls, got %d", len(store.getCalls))
	}
	if store.getCalls[0] != "Statham Meme Quotes" {
		t.Fatalf("first get call: got %q", store.getCalls[0])
	}
	if store.getCalls[1] != "statham-meme-quotes" {
		t.Fatalf("second get call: got %q", store.getCalls[1])
	}
	if len(store.loadCalls) != 1 || store.loadCalls[0] != "statham-meme-quotes" {
		t.Fatalf("expected load by slug, got %v", store.loadCalls)
	}
	// LOCAL FIX END: ensure skills.get resolves display names through the listed slug
}

// ---- Tests: handleUpdate ----

func TestSkillsUpdate_MissingNameAndID_ReturnsInvalidRequest(t *testing.T) {
	m := buildSkillMethods(t, nil)
	client := nullClient()
	// Neither name nor id provided
	req := skillReqFrame(t, protocol.MethodSkillsUpdate, map[string]any{
		"updates": map[string]any{"enabled": true},
	})
	m.handleUpdate(context.Background(), client, req)
	// No panic = invalid-request path hit
}

func TestSkillsUpdate_StoreNotUpdater_ReturnsNotFound(t *testing.T) {
	// stubSkillStore does NOT implement skillUpdater — update should return ErrNotFound
	skills := []store.SkillInfo{{Name: "some-skill", Slug: "some-skill"}}
	m := buildSkillMethods(t, skills)
	client := nullClient()
	req := skillReqFrame(t, protocol.MethodSkillsUpdate, map[string]any{
		"name":    "some-skill",
		"updates": map[string]any{"enabled": false},
	})
	m.handleUpdate(context.Background(), client, req)
	// No panic = "store does not support updates" path hit
}

func TestSkillsUpdate_EmptyUpdates_ReturnsInvalidRequest(t *testing.T) {
	// name provided but updates is empty
	skills := []store.SkillInfo{{Name: "sk", Slug: "sk"}}
	m := buildSkillMethods(t, skills)
	client := nullClient()
	req := skillReqFrame(t, protocol.MethodSkillsUpdate, map[string]any{
		"name":    "sk",
		"updates": map[string]any{},
	})
	m.handleUpdate(context.Background(), client, req)
	// No panic = invalid-request (empty updates) path hit
}
