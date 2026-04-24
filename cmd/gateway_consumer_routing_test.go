package cmd

import (
	"testing"

	"github.com/nextlevelbuilder/goclaw/internal/bus"
	"github.com/nextlevelbuilder/goclaw/internal/channels"
)

// LOCAL FIX START: telegram reply target preservation tests
func TestBuildFinalOutboundMeta_AddsTelegramReplyTargetForDefaultChannel(t *testing.T) {
	msg := bus.InboundMessage{
		Channel: channels.TypeTelegram,
		Metadata: map[string]string{
			"message_id":        "123",
			"message_thread_id": "45",
			"local_key":         "-1001:topic:45",
		},
	}

	got := buildFinalOutboundMeta(msg, "")

	if got["reply_to_message_id"] != "123" {
		t.Fatalf("reply_to_message_id = %q, want 123", got["reply_to_message_id"])
	}
	if got["message_thread_id"] != "45" {
		t.Fatalf("message_thread_id = %q, want 45", got["message_thread_id"])
	}
	if got["local_key"] != "-1001:topic:45" {
		t.Fatalf("local_key = %q, want -1001:topic:45", got["local_key"])
	}
}

func TestBuildFinalOutboundMeta_AddsTelegramReplyTargetForInstanceChannel(t *testing.T) {
	msg := bus.InboundMessage{
		Channel: "bibbopcoderbot",
		Metadata: map[string]string{
			"message_id": "321",
		},
	}

	got := buildFinalOutboundMeta(msg, channels.TypeTelegram)

	if got["reply_to_message_id"] != "321" {
		t.Fatalf("reply_to_message_id = %q, want 321", got["reply_to_message_id"])
	}
}

func TestBuildFinalOutboundMeta_DoesNotAddReplyTargetForOtherChannels(t *testing.T) {
	msg := bus.InboundMessage{
		Channel: "slack-main",
		Metadata: map[string]string{
			"message_id": "999",
		},
	}

	got := buildFinalOutboundMeta(msg, channels.TypeSlack)

	if got["reply_to_message_id"] != "" {
		t.Fatalf("reply_to_message_id = %q, want empty", got["reply_to_message_id"])
	}
}

// LOCAL FIX END: telegram reply target preservation tests
