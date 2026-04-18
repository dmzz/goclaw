package telegram

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/mymmrac/telego"
)

// buildSelfIdentityPrompt returns a short system-prompt snippet telling the LLM
// which Telegram handle represents itself, so it does not confuse its own
// @mention for a different bot — especially useful in multi-bot groups where
// other bots' mentions remain in the content after stripBotMention.
// Returns empty string when the bot username has not been resolved yet.
func buildSelfIdentityPrompt(botUsername, displayName string) string {
	if botUsername == "" {
		return ""
	}
	if displayName != "" {
		return fmt.Sprintf("You are @%s (%s) on this Telegram channel.", botUsername, displayName)
	}
	return fmt.Sprintf("You are @%s on this Telegram channel.", botUsername)
}

// stripBotMention removes @botUsername tokens from text (case-insensitive).
// Applied after the mention gate passes so the LLM does not see its own Telegram handle
// and mistake itself for another bot (e.g. persona "Tiểu Hổ" receiving "@viet_super_bot vẽ...").
//
// Boundary rules match valid Telegram mentions:
//   - Leading: start-of-string OR a non-word char (whitespace/punct). Prevents false strips
//     inside words like "contact@viet_super_bot.com".
//   - Trailing: \b (word-boundary). Prevents matching "@bot" inside "@bot_2".
//
// The leading non-word char is preserved via capture group $1.
func stripBotMention(text, botUsername string) string {
	if botUsername == "" || text == "" {
		return text
	}
	pattern := `(?i)(^|[^\w])@` + regexp.QuoteMeta(botUsername) + `\b`
	return strings.TrimSpace(regexp.MustCompile(pattern).ReplaceAllString(text, "$1"))
}

// detectMention checks if a Telegram message mentions the bot.
// Checks both msg.Text/Entities (text messages) and msg.Caption/CaptionEntities (photo/media messages).
func (c *Channel) detectMention(msg *telego.Message, botUsername string) bool {
	if botUsername == "" {
		return false
	}

	// Check both text entities and caption entities (photos use Caption, not Text).
	for _, pair := range []struct {
		entities []telego.MessageEntity
		text     string
	}{
		{msg.Entities, msg.Text},
		{msg.CaptionEntities, msg.Caption},
	} {
		if pair.text == "" {
			continue
		}
		for _, entity := range pair.entities {
			if entity.Type != "mention" && entity.Type != "bot_command" {
				continue
			}
			if entityText, ok := telegramEntityText(pair.text, entity); ok && textTargetsTelegramHandle(entityText, botUsername) {
				return true
			}
		}
	}

	// Fallback: parse visible text in both text and caption. This keeps
	// mention detection working even when entities are absent or malformed.
	if textTargetsTelegramHandle(msg.Text, botUsername) {
		return true
	}
	if textTargetsTelegramHandle(msg.Caption, botUsername) {
		return true
	}

	// Reply to bot's message = implicit mention
	if msg.ReplyToMessage != nil && msg.ReplyToMessage.From != nil {
		if strings.EqualFold(msg.ReplyToMessage.From.Username, botUsername) {
			return true
		}
	}

	return false
}

// hasOtherMention checks if the message mentions any user/bot other than ours.
// Used by "yield" mention mode: if another entity is explicitly @mentioned, this bot yields.
// Checks both "mention" entities (@username) and "bot_command" entities (/cmd@bot).
func (c *Channel) hasOtherMention(msg *telego.Message, myUsername string) bool {
	for _, pair := range []struct {
		entities []telego.MessageEntity
		text     string
	}{
		{msg.Entities, msg.Text},
		{msg.CaptionEntities, msg.Caption},
	} {
		if pair.text == "" {
			continue
		}
		for _, entity := range pair.entities {
			if entity.Type == "mention" {
				mentioned, ok := telegramEntityText(pair.text, entity)
				if !ok {
					continue
				}
				if !strings.EqualFold(strings.TrimPrefix(mentioned, "@"), myUsername) {
					return true
				}
			}
			if entity.Type == "bot_command" {
				cmdText, ok := telegramEntityText(pair.text, entity)
				if !ok {
					continue
				}
				for _, target := range extractTelegramCommandTargets(cmdText) {
					if !strings.EqualFold(target, myUsername) {
						return true
					}
				}
			}
		}
		if hasOtherTelegramHandle(pair.text, myUsername) {
			return true
		}
	}
	return false
}

// shouldSkipForeignBotMessageInYield applies Telegram's multi-bot inbound guard.
// Default behavior skips other bots unless they explicitly target us. When
// allowBotMessages is enabled, standalone bot messages are allowed through but
// bot-to-bot reply chains still yield to avoid ping-pong loops.
func (c *Channel) shouldSkipForeignBotMessageInYield(msg *telego.Message, myUsername string, allowBotMessages bool) bool {
	if msg == nil || msg.From == nil || !msg.From.IsBot || strings.EqualFold(msg.From.Username, myUsername) {
		return false
	}
	if c.detectMention(msg, myUsername) {
		return false
	}
	if !allowBotMessages {
		return true
	}
	return isReplyToOtherBot(msg, myUsername)
}

// isReplyToOtherBot returns true when the message replies to a bot other than ours.
// Used to keep yield-mode bot-to-bot loop protection even when bot messages are allowed.
func isReplyToOtherBot(msg *telego.Message, myUsername string) bool {
	if msg == nil || msg.ReplyToMessage == nil || msg.ReplyToMessage.From == nil {
		return false
	}
	replyFrom := msg.ReplyToMessage.From
	return replyFrom.IsBot && !strings.EqualFold(replyFrom.Username, myUsername)
}

// isServiceMessage returns true if the Telegram message is a service/system message
// (member added/removed, title changed, pinned, etc.) rather than a user-sent message.
// Service messages have no text, caption, or media content.
func isServiceMessage(msg *telego.Message) bool {
	// Has text or caption → user message
	if msg.Text != "" || msg.Caption != "" {
		return false
	}

	// Has media → user message (photo, audio, video, document, sticker, etc.)
	if msg.Photo != nil || msg.Audio != nil || msg.Video != nil ||
		msg.Document != nil || msg.Voice != nil || msg.VideoNote != nil ||
		msg.Sticker != nil || msg.Animation != nil || msg.Contact != nil ||
		msg.Location != nil || msg.Venue != nil || msg.Poll != nil {
		return false
	}

	// No user content — likely a service message (new_chat_members, left_chat_member,
	// new_chat_title, new_chat_photo, pinned_message, etc.)
	return true
}
