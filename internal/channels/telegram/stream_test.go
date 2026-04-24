package telegram

import "testing"

// LOCAL FIX START: telegram reply target preservation tests
func TestTelegramReplyToMessageID(t *testing.T) {
	tests := []struct {
		name     string
		metadata map[string]string
		want     int
	}{
		{
			name:     "valid reply target",
			metadata: map[string]string{"reply_to_message_id": "123"},
			want:     123,
		},
		{
			name:     "missing metadata",
			metadata: nil,
			want:     0,
		},
		{
			name:     "non numeric",
			metadata: map[string]string{"reply_to_message_id": "abc"},
			want:     0,
		},
		{
			name:     "non positive",
			metadata: map[string]string{"reply_to_message_id": "0"},
			want:     0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := telegramReplyToMessageID(tt.metadata); got != tt.want {
				t.Fatalf("telegramReplyToMessageID() = %d, want %d", got, tt.want)
			}
		})
	}
}

func TestNewDraftStreamStoresReplyTarget(t *testing.T) {
	ds := NewDraftStream(nil, -1001, 0, 42, false, 123)

	if ds.replyToMessageID != 123 {
		t.Fatalf("replyToMessageID = %d, want 123", ds.replyToMessageID)
	}

	params := ds.newSendMessageParams("hello")
	if params.ReplyParameters == nil {
		t.Fatal("ReplyParameters is nil, want reply target")
	}
	if params.ReplyParameters.MessageID != 123 {
		t.Fatalf("ReplyParameters.MessageID = %d, want 123", params.ReplyParameters.MessageID)
	}
	if !params.ReplyParameters.AllowSendingWithoutReply {
		t.Fatal("ReplyParameters.AllowSendingWithoutReply = false, want true")
	}
}

// LOCAL FIX END: telegram reply target preservation tests
