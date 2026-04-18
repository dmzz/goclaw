package telegram

import (
	"reflect"
	"testing"

	"github.com/mymmrac/telego"
)

func TestSliceByUTF16CodeUnits_CyrillicPrefix(t *testing.T) {
	got, ok := sliceByUTF16CodeUnits("Привет @otherbot", 7, 9)
	if !ok {
		t.Fatal("sliceByUTF16CodeUnits returned ok=false")
	}
	if got != "@otherbot" {
		t.Fatalf("sliceByUTF16CodeUnits = %q, want %q", got, "@otherbot")
	}
}

func TestSliceByUTF16CodeUnits_EmojiPrefix(t *testing.T) {
	got, ok := sliceByUTF16CodeUnits("🙂 @mybot", 3, 6)
	if !ok {
		t.Fatal("sliceByUTF16CodeUnits returned ok=false")
	}
	if got != "@mybot" {
		t.Fatalf("sliceByUTF16CodeUnits = %q, want %q", got, "@mybot")
	}
}

func TestTelegramEntityText_UTF16Offsets(t *testing.T) {
	entity := telego.MessageEntity{Type: "mention", Offset: 7, Length: 9}
	got, ok := telegramEntityText("Привет @otherbot", entity)
	if !ok {
		t.Fatal("telegramEntityText returned ok=false")
	}
	if got != "@otherbot" {
		t.Fatalf("telegramEntityText = %q, want %q", got, "@otherbot")
	}
}

func TestExtractTelegramMentionHandles_DedupAndEmailSafe(t *testing.T) {
	got := ExtractTelegramMentionHandles("ask @OtherBot, then @helper_bot. mail me at user@example.com. repeat @otherbot")
	want := []string{"@OtherBot", "@helper_bot"}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("ExtractTelegramMentionHandles = %#v, want %#v", got, want)
	}
}

func TestTextTargetsTelegramHandle_CommandTarget(t *testing.T) {
	if !textTargetsTelegramHandle("/start@HelperBot", "helperbot") {
		t.Fatal("textTargetsTelegramHandle should match /cmd@bot")
	}
}

func TestHasOtherTelegramHandle_UnicodeContext(t *testing.T) {
	if !hasOtherTelegramHandle("Привет @otherbot", "mybot") {
		t.Fatal("hasOtherTelegramHandle should detect another mention with unicode prefix")
	}
}

func TestDetectMention_UnicodePrefixEntity(t *testing.T) {
	ch := &Channel{}
	msg := &telego.Message{
		Text: "Привет @mybot",
		Entities: []telego.MessageEntity{
			{Type: "mention", Offset: 7, Length: 6},
		},
	}
	if !ch.detectMention(msg, "mybot") {
		t.Fatal("detectMention should match @mybot after unicode prefix")
	}
}

func TestHasOtherMention_UnicodePrefixEntity(t *testing.T) {
	ch := &Channel{}
	msg := &telego.Message{
		Text: "Привет @otherbot",
		Entities: []telego.MessageEntity{
			{Type: "mention", Offset: 7, Length: 9},
		},
	}
	if !ch.hasOtherMention(msg, "mybot") {
		t.Fatal("hasOtherMention should detect another bot after unicode prefix")
	}
}
