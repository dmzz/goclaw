package cmd

import (
	"fmt"
	"strings"

	"github.com/nextlevelbuilder/goclaw/internal/channels/telegram"
)

// LOCAL FIX START: telegram multi-bot prompt augmentation
func augmentTelegramGroupPrompt(extraPrompt string) string {
	lines := []string{
		"- In Telegram groups with multiple bots, explicitly mention the target bot as @username when you want that bot to act.",
		"- If you want another bot to reply, keep its exact @handle verbatim in your outgoing message.",
	}

	if handles := telegram.ExtractTelegramMentionHandles(extraPrompt); len(handles) > 0 {
		lines = append(lines, fmt.Sprintf("- Known Telegram handles from your instructions: %s.", strings.Join(handles, ", ")))
		lines = append(lines, "- Do not paraphrase, translate, or drop those @handles when handing off work to them.")
	}

	block := "Telegram Multi-Bot Routing:\n" + strings.Join(lines, "\n")
	if extraPrompt == "" {
		return block
	}
	return extraPrompt + "\n\n" + block
}

// LOCAL FIX END: telegram multi-bot prompt augmentation
