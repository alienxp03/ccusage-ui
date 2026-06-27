package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestExtractLastUserMessageFromSupportedJSONLFormats(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	content := `{"type":"message","timestamp":"2026-06-25T10:00:00Z","message":{"role":"user","content":[{"type":"text","text":"first pi message"}]}}
{"timestamp":"2026-06-25T11:00:00Z","type":"response_item","payload":{"type":"message","role":"user","content":[{"type":"input_text","text":"second codex message"}]}}
{"type":"user","timestamp":"2026-06-25T12:00:00Z","content":"third claude message"}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	preview, timestamp, err := extractLastUserMessage(path)
	if err != nil {
		t.Fatal(err)
	}

	if preview != "third claude message" {
		t.Fatalf("expected last preview, got %q", preview)
	}
	if timestamp != "2026-06-25T12:00:00Z" {
		t.Fatalf("expected timestamp, got %q", timestamp)
	}
}

func TestExtractConversationMessagesFromClaudeJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	content := `{"type":"user","timestamp":"2026-06-25T10:00:00Z","content":"hello claude"}
{"type":"tool_use","timestamp":"2026-06-25T10:00:01Z","tool_name":"read"}
{"type":"assistant","timestamp":"2026-06-25T10:00:02Z","content":"hello user"}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	messages, err := extractConversationMessages(path)
	if err != nil {
		t.Fatal(err)
	}

	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != "user" || messages[0].Text != "hello claude" {
		t.Fatalf("unexpected user message: %#v", messages[0])
	}
	if messages[1].Role != "assistant" || messages[1].Text != "hello user" {
		t.Fatalf("unexpected assistant message: %#v", messages[1])
	}
}
