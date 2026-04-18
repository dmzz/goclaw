package cmd

import (
	"strings"
	"testing"
)

func TestAugmentTelegramGroupPrompt_AddsRoutingGuidance(t *testing.T) {
	got := augmentTelegramGroupPrompt("Base prompt")
	if !strings.Contains(got, "Telegram Multi-Bot Routing:") {
		t.Fatalf("augmentTelegramGroupPrompt() missing routing block: %q", got)
	}
	if !strings.Contains(got, "explicitly mention the target bot as @username") {
		t.Fatalf("augmentTelegramGroupPrompt() missing explicit mention guidance: %q", got)
	}
}

func TestAugmentTelegramGroupPrompt_ListsKnownHandles(t *testing.T) {
	got := augmentTelegramGroupPrompt("Ask @OtherBot first, then ask @helper_bot. Ignore user@example.com.")
	wantLine := "Known Telegram handles from your instructions: @OtherBot, @helper_bot."
	if !strings.Contains(got, wantLine) {
		t.Fatalf("augmentTelegramGroupPrompt() missing known handles list: %q", got)
	}
	for _, line := range strings.Split(got, "\n") {
		if strings.Contains(line, "Known Telegram handles from your instructions:") && strings.Contains(line, "@example") {
			t.Fatalf("augmentTelegramGroupPrompt() should not treat emails as handles: %q", got)
		}
	}
}
