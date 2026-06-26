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
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	preview, timestamp, err := extractLastUserMessage(path)
	if err != nil {
		t.Fatal(err)
	}

	if preview != "second codex message" {
		t.Fatalf("expected last preview, got %q", preview)
	}
	if timestamp != "2026-06-25T11:00:00Z" {
		t.Fatalf("expected timestamp, got %q", timestamp)
	}
}
