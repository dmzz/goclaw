package providers

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
)

// newOpenAISSEServer creates a mock SSE server for OpenAI-compatible streaming.
func newOpenAISSEServer(t *testing.T, chunks []string) *httptest.Server {
	t.Helper()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Error("ResponseWriter does not implement http.Flusher")
			return
		}
		for _, chunk := range chunks {
			fmt.Fprint(w, chunk)
			flusher.Flush()
		}
	}))
	t.Cleanup(server.Close)
	return server
}

func newTestOpenAIProvider(baseURL string) *OpenAIProvider {
	p := NewOpenAIProvider("test", "test-key", baseURL, "gpt-4")
	p.retryConfig.Attempts = 1
	return p
}

// TestChatStream_TruncatedToolCallArgs verifies that when a stream is cut mid-JSON
// (finish_reason: "length"), FinishReason is preserved as "length" and ParseError is set.
func TestChatStream_TruncatedToolCallArgs(t *testing.T) {
	chunks := []string{
		// Tool call with partial arguments (truncated JSON)
		`data: {"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_abc123","type":"function","function":{"name":"write_file","arguments":""}}]}}]}` + "\n\n",
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"path\":\"/tmp/test.txt\",\"content\":\"hello wor"}}]}}]}` + "\n\n",
		// Stream truncated — finish_reason: "length"
		`data: {"choices":[{"index":0,"finish_reason":"length","delta":{}}]}` + "\n\n",
		"data: [DONE]\n\n",
	}

	server := newOpenAISSEServer(t, chunks)
	p := newTestOpenAIProvider(server.URL)

	req := ChatRequest{
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "write a file"}},
	}
	result, err := p.ChatStream(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// FinishReason must be "length", NOT "tool_calls"
	if result.FinishReason != "length" {
		t.Errorf("FinishReason = %q, want %q", result.FinishReason, "length")
	}

	// Tool call should still be present (for logging) but with ParseError set
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	tc := result.ToolCalls[0]
	if tc.ParseError == "" {
		t.Error("expected ParseError to be set for truncated JSON args")
	}
	if tc.Name != "write_file" {
		t.Errorf("tool name = %q, want %q", tc.Name, "write_file")
	}
	// Arguments should be empty map (parse failed)
	if len(tc.Arguments) != 0 {
		t.Errorf("expected empty args for truncated JSON, got %v", tc.Arguments)
	}
}

// TestChatStream_CompleteToolCallArgs verifies that normal (non-truncated) tool calls
// still get FinishReason = "tool_calls" and no ParseError.
func TestChatStream_CompleteToolCallArgs(t *testing.T) {
	chunks := []string{
		`data: {"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_ok","type":"function","function":{"name":"read_file","arguments":""}}]}}]}` + "\n\n",
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"path\":\"/tmp/test.txt\"}"}}]}}]}` + "\n\n",
		`data: {"choices":[{"index":0,"finish_reason":"stop","delta":{}}]}` + "\n\n",
		"data: [DONE]\n\n",
	}

	server := newOpenAISSEServer(t, chunks)
	p := newTestOpenAIProvider(server.URL)

	req := ChatRequest{
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "read a file"}},
	}
	result, err := p.ChatStream(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Normal tool call: FinishReason should be "tool_calls"
	if result.FinishReason != "tool_calls" {
		t.Errorf("FinishReason = %q, want %q", result.FinishReason, "tool_calls")
	}

	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	tc := result.ToolCalls[0]
	if tc.ParseError != "" {
		t.Errorf("expected no ParseError, got %q", tc.ParseError)
	}
	if tc.Arguments["path"] != "/tmp/test.txt" {
		t.Errorf("expected path=/tmp/test.txt, got %v", tc.Arguments["path"])
	}
}

func TestChatStream_CumulativeToolCallArgs(t *testing.T) {
	chunks := []string{
		`data: {"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_web","type":"function","function":{"name":"web_search","arguments":"{\"query\":\"AI"}}]}}]}` + "\n\n",
		// Provider resends the full accumulated JSON instead of just the delta.
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"query\":\"AI agents news\",\"count\":5}"}}]}}]}` + "\n\n",
		`data: {"choices":[{"index":0,"finish_reason":"tool_calls","delta":{}}]}` + "\n\n",
		"data: [DONE]\n\n",
	}

	server := newOpenAISSEServer(t, chunks)
	p := newTestOpenAIProvider(server.URL)

	result, err := p.ChatStream(context.Background(), ChatRequest{
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "search the web"}},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinishReason != "tool_calls" {
		t.Fatalf("FinishReason = %q, want %q", result.FinishReason, "tool_calls")
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	tc := result.ToolCalls[0]
	if tc.ParseError != "" {
		t.Fatalf("unexpected ParseError: %q", tc.ParseError)
	}
	if got := tc.Arguments["query"]; got != "AI agents news" {
		t.Fatalf("query = %v, want %q", got, "AI agents news")
	}
	if got := tc.Arguments["count"]; got != float64(5) {
		t.Fatalf("count = %v, want 5", got)
	}
}

func TestMergeOpenAIStreamToolArgs_Overlap(t *testing.T) {
	got := mergeOpenAIStreamToolArgs(`{"query":"AI agen`, `agents news","count":5}`)
	want := `{"query":"AI agents news","count":5}`
	if got != want {
		t.Fatalf("mergeOpenAIStreamToolArgs() = %q, want %q", got, want)
	}
}

func TestDecodeOpenAIToolArgs_ConcatenatedObjects(t *testing.T) {
	got, recoveredBy, err := decodeOpenAIToolArgs(`{"query":"AI agents news","count":5}{"query":"AI agents news this week","count":10}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recoveredBy != "json_sequence_last" {
		t.Fatalf("recoveredBy = %q, want %q", recoveredBy, "json_sequence_last")
	}
	if got["query"] != "AI agents news this week" {
		t.Fatalf("query = %v, want %q", got["query"], "AI agents news this week")
	}
	if got["count"] != float64(10) {
		t.Fatalf("count = %v, want 10", got["count"])
	}
}

func TestDecodeOpenAIToolArgs_ResyncsToValidSuffix(t *testing.T) {
	got, recoveredBy, err := decodeOpenAIToolArgs(`{"query":"AI agents news","count":5}{"query":"AI agents news this week","count":10`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recoveredBy != "json_sequence_last_partial" {
		t.Fatalf("recoveredBy = %q, want %q", recoveredBy, "json_sequence_last_partial")
	}
	if got["query"] != "AI agents news" {
		t.Fatalf("query = %v, want %q", got["query"], "AI agents news")
	}
	if got["count"] != float64(5) {
		t.Fatalf("count = %v, want 5", got["count"])
	}
}

func TestDecodeOpenAIToolArgs_SuffixResync(t *testing.T) {
	got, recoveredBy, err := decodeOpenAIToolArgs(`garbage-prefix {"query":"AI agents news","count":5}`)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if recoveredBy != "suffix_resync" {
		t.Fatalf("recoveredBy = %q, want %q", recoveredBy, "suffix_resync")
	}
	if got["query"] != "AI agents news" {
		t.Fatalf("query = %v, want %q", got["query"], "AI agents news")
	}
}

func TestChatStream_ConcatenatedToolCallArgs(t *testing.T) {
	chunks := []string{
		`data: {"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_web","type":"function","function":{"name":"web_search","arguments":"{\"query\":\"AI agents news\",\"count\":5}"}}]}}]}` + "\n\n",
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"query\":\"AI agents news this week\",\"count\":10}"}}]}}]}` + "\n\n",
		`data: {"choices":[{"index":0,"finish_reason":"tool_calls","delta":{}}]}` + "\n\n",
		"data: [DONE]\n\n",
	}

	server := newOpenAISSEServer(t, chunks)
	p := newTestOpenAIProvider(server.URL)

	result, err := p.ChatStream(context.Background(), ChatRequest{
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "search the web"}},
	}, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.FinishReason != "tool_calls" {
		t.Fatalf("FinishReason = %q, want %q", result.FinishReason, "tool_calls")
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	tc := result.ToolCalls[0]
	if tc.ParseError != "" {
		t.Fatalf("unexpected ParseError: %q", tc.ParseError)
	}
	if tc.Arguments["query"] != "AI agents news this week" {
		t.Fatalf("query = %v, want %q", tc.Arguments["query"], "AI agents news this week")
	}
	if tc.Arguments["count"] != float64(10) {
		t.Fatalf("count = %v, want 10", tc.Arguments["count"])
	}
}

// TestChatStream_MultipleToolCalls_OneTruncated verifies that when one tool call
// has valid args and another is truncated, ParseError is set only on the truncated one.
func TestChatStream_MultipleToolCalls_OneTruncated(t *testing.T) {
	chunks := []string{
		// Tool 0: complete args
		`data: {"choices":[{"index":0,"delta":{"role":"assistant","tool_calls":[{"index":0,"id":"call_a","type":"function","function":{"name":"read_file","arguments":""}}]}}]}` + "\n\n",
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":0,"function":{"arguments":"{\"path\":\"ok.txt\"}"}}]}}]}` + "\n\n",
		// Tool 1: truncated args
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"id":"call_b","type":"function","function":{"name":"write_file","arguments":""}}]}}]}` + "\n\n",
		`data: {"choices":[{"index":0,"delta":{"tool_calls":[{"index":1,"function":{"arguments":"{\"path\":\"big.txt\",\"content\":\"trunc"}}]}}]}` + "\n\n",
		// Truncated
		`data: {"choices":[{"index":0,"finish_reason":"length","delta":{}}]}` + "\n\n",
		"data: [DONE]\n\n",
	}

	server := newOpenAISSEServer(t, chunks)
	p := newTestOpenAIProvider(server.URL)

	req := ChatRequest{
		Model:    "gpt-4",
		Messages: []Message{{Role: "user", Content: "multi tool"}},
	}
	result, err := p.ChatStream(context.Background(), req, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if result.FinishReason != "length" {
		t.Errorf("FinishReason = %q, want %q", result.FinishReason, "length")
	}
	if len(result.ToolCalls) != 2 {
		t.Fatalf("expected 2 tool calls, got %d", len(result.ToolCalls))
	}

	// Tool 0 should be valid
	if result.ToolCalls[0].ParseError != "" {
		t.Errorf("tool 0: unexpected ParseError %q", result.ToolCalls[0].ParseError)
	}
	// Tool 1 should have parse error
	if result.ToolCalls[1].ParseError == "" {
		t.Error("tool 1: expected ParseError for truncated args")
	}
}

// TestParseResponse_TruncatedToolCallArgs verifies the non-streaming path
// preserves FinishReason "length" and sets ParseError.
func TestParseResponse_TruncatedToolCallArgs(t *testing.T) {
	p := NewOpenAIProvider("test", "key", "https://api.openai.com/v1", "gpt-4")

	// Simulate a non-streaming response with truncated tool call args
	resp := &openAIResponse{
		Choices: []openAIChoice{
			{
				FinishReason: "length",
				Message: openAIMessage{
					Role: "assistant",
					ToolCalls: []openAIToolCall{
						{
							ID:   "call_trunc",
							Type: "function",
							Function: openAIFunctionCall{
								Name:      "write_file",
								Arguments: `{"path":"/tmp/x","content":"hello wor`, // truncated
							},
						},
					},
				},
			},
		},
	}

	result := p.parseResponse(resp)

	// FinishReason must stay "length"
	if result.FinishReason != "length" {
		t.Errorf("FinishReason = %q, want %q", result.FinishReason, "length")
	}

	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].ParseError == "" {
		t.Error("expected ParseError for truncated JSON")
	}
}

// TestParseResponse_ValidToolCallArgs verifies the non-streaming path
// overrides FinishReason to "tool_calls" when args are valid.
func TestParseResponse_ValidToolCallArgs(t *testing.T) {
	p := NewOpenAIProvider("test", "key", "https://api.openai.com/v1", "gpt-4")

	resp := &openAIResponse{
		Choices: []openAIChoice{
			{
				FinishReason: "stop",
				Message: openAIMessage{
					Role: "assistant",
					ToolCalls: []openAIToolCall{
						{
							ID:   "call_ok",
							Type: "function",
							Function: openAIFunctionCall{
								Name:      "read_file",
								Arguments: `{"path":"/tmp/test.txt"}`,
							},
						},
					},
				},
			},
		},
	}

	result := p.parseResponse(resp)

	if result.FinishReason != "tool_calls" {
		t.Errorf("FinishReason = %q, want %q", result.FinishReason, "tool_calls")
	}
	if len(result.ToolCalls) != 1 {
		t.Fatalf("expected 1 tool call, got %d", len(result.ToolCalls))
	}
	if result.ToolCalls[0].ParseError != "" {
		t.Errorf("unexpected ParseError: %q", result.ToolCalls[0].ParseError)
	}
}
