package telegram

import (
	"regexp"
	"strings"

	"github.com/mymmrac/telego"
)

var (
	telegramMentionPattern          = regexp.MustCompile(`(^|[^A-Za-z0-9_])(@[A-Za-z][A-Za-z0-9_]{4,31})([^A-Za-z0-9_]|$)`)
	telegramBotCommandTargetPattern = regexp.MustCompile(`(^|[^A-Za-z0-9_])/[A-Za-z0-9_]+@([A-Za-z][A-Za-z0-9_]{4,31})([^A-Za-z0-9_]|$)`)
)

// ExtractTelegramMentionHandles returns unique explicit @handles found in text.
// Handles preserve their first-seen spelling and order.
func ExtractTelegramMentionHandles(text string) []string {
	return extractTelegramMentionHandles(text)
}

func extractTelegramMentionHandles(text string) []string {
	if text == "" {
		return nil
	}
	var out []string
	seen := make(map[string]struct{})
	for _, match := range telegramMentionPattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 3 {
			continue
		}
		handle := match[2]
		lower := strings.ToLower(handle)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		out = append(out, handle)
	}
	return out
}

func extractTelegramCommandTargets(text string) []string {
	if text == "" {
		return nil
	}
	var out []string
	seen := make(map[string]struct{})
	for _, match := range telegramBotCommandTargetPattern.FindAllStringSubmatch(text, -1) {
		if len(match) < 3 {
			continue
		}
		target := match[2]
		lower := strings.ToLower(target)
		if _, ok := seen[lower]; ok {
			continue
		}
		seen[lower] = struct{}{}
		out = append(out, target)
	}
	return out
}

func textTargetsTelegramHandle(text, username string) bool {
	if text == "" || username == "" {
		return false
	}
	want := "@" + strings.ToLower(username)
	for _, handle := range extractTelegramMentionHandles(text) {
		if strings.ToLower(handle) == want {
			return true
		}
	}
	for _, target := range extractTelegramCommandTargets(text) {
		if strings.EqualFold(target, username) {
			return true
		}
	}
	return false
}

func hasOtherTelegramHandle(text, myUsername string) bool {
	lowerMy := strings.ToLower(myUsername)
	for _, handle := range extractTelegramMentionHandles(text) {
		if strings.TrimPrefix(strings.ToLower(handle), "@") != lowerMy {
			return true
		}
	}
	for _, target := range extractTelegramCommandTargets(text) {
		if strings.ToLower(target) != lowerMy {
			return true
		}
	}
	return false
}

func telegramEntityText(text string, entity telego.MessageEntity) (string, bool) {
	return sliceByUTF16CodeUnits(text, entity.Offset, entity.Length)
}

func sliceByUTF16CodeUnits(text string, offset, length int) (string, bool) {
	if offset < 0 || length < 0 {
		return "", false
	}

	targetEnd := offset + length
	startByte := -1
	endByte := -1
	codeUnits := 0

	for idx, r := range text {
		if codeUnits == offset && startByte == -1 {
			startByte = idx
		}
		if codeUnits == targetEnd && endByte == -1 {
			endByte = idx
		}

		width := 1
		if r > 0xFFFF {
			width = 2
		}
		nextUnits := codeUnits + width

		if offset > codeUnits && offset < nextUnits {
			return "", false
		}
		if targetEnd > codeUnits && targetEnd < nextUnits {
			return "", false
		}

		codeUnits = nextUnits
	}

	if codeUnits == offset && startByte == -1 {
		startByte = len(text)
	}
	if codeUnits == targetEnd && endByte == -1 {
		endByte = len(text)
	}
	if startByte < 0 || endByte < 0 || startByte > endByte {
		return "", false
	}

	return text[startByte:endByte], true
}
