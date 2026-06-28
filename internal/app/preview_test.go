package app

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

func TestGeminiSessionFragment(t *testing.T) {
	cases := map[string]string{
		"ffcd9379-d7cf-468c-b199-922d2d92aa8e": "ffcd9379",
		"eea29656-6dd5-4ba1-b351-0a27017fa18d": "eea29656",
		"short-id":                             "short-id",
		"":                                     "",
	}
	for input, want := range cases {
		if got := geminiSessionFragment(input); got != want {
			t.Errorf("geminiSessionFragment(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestExportGeminiSession(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "session-2026-01-23T09-17-ffcd9379.json")
	content := `{
		"sessionId": "ffcd9379-d7cf-468c-b199-922d2d92aa8e",
		"projectHash": "projecthash",
		"startTime": "2026-01-23T09:17:36.037Z",
		"lastUpdated": "2026-01-23T09:17:58.271Z",
		"messages": [
			{"id":"m1","timestamp":"2026-01-23T09:17:36.100Z","type":"user","content":"summarize the options"},
			{"id":"m2","timestamp":"2026-01-23T09:17:58.271Z","type":"gemini","content":"here is the evaluation"}
		]
	}`
	if err := os.WriteFile(source, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}

	transcript := filepath.Join(dir, "out.jsonl")
	if err := exportGeminiSession(source, transcript); err != nil {
		t.Fatal(err)
	}

	messages, err := extractConversationMessages(transcript)
	if err != nil {
		t.Fatal(err)
	}
	if len(messages) != 2 {
		t.Fatalf("expected 2 messages, got %d", len(messages))
	}
	if messages[0].Role != "user" || messages[0].Text != "summarize the options" {
		t.Fatalf("unexpected user message: %#v", messages[0])
	}
	if messages[1].Role != "assistant" || messages[1].Text != "here is the evaluation" {
		t.Fatalf("expected gemini type mapped to assistant: %#v", messages[1])
	}
}

func TestExportGeminiSessionEmpty(t *testing.T) {
	dir := t.TempDir()
	source := filepath.Join(dir, "empty.json")
	if err := os.WriteFile(source, []byte(`{"sessionId":"x","messages":[]}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := exportGeminiSession(source, filepath.Join(dir, "out.jsonl")); err == nil {
		t.Fatal("expected error for session with no conversation messages")
	}
}

func TestReadGeminiSessionMeta(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	chats := filepath.Join(home, ".gemini", "tmp", "projecthash", "chats")
	if err := os.MkdirAll(chats, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chats, "session-2026-01-23T09-17-ffcd9379.json"), []byte(`{"sessionId":"ffcd9379-d7cf-468c-b199-922d2d92aa8e","lastUpdated":"2026-01-23T09:17:58.271Z"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	// Non-session files (e.g. logs.json) must be ignored.
	if err := os.WriteFile(filepath.Join(chats, "logs.json"), []byte(`{}`), 0o600); err != nil {
		t.Fatal(err)
	}

	meta := readGeminiSessionMeta()
	entry, ok := meta["ffcd9379-d7cf-468c-b199-922d2d92aa8e"]
	if !ok {
		t.Fatalf("expected gemini session meta mapped, got %#v", meta)
	}
	if entry.lastActivity != "2026-01-23T09:17:58.271Z" {
		t.Fatalf("expected last activity, got %q", entry.lastActivity)
	}
	if entry.projectHash != "projecthash" {
		t.Fatalf("expected project hash from tmp dir name, got %q", entry.projectHash)
	}
}

func TestResolveGeminiProjectPaths(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	// A real project on disk under a nested dev root (resolved via the walk).
	projectDir := filepath.Join(home, "Workspace", "Projects", "conclave")
	if err := os.MkdirAll(projectDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A heavy tree that must be pruned, not descended into.
	heavyDir := filepath.Join(home, "Workspace", "Projects", "skipped", "node_modules", "pkg")
	if err := os.MkdirAll(heavyDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// A seed path recorded in projects.json (resolved without relying on the walk).
	seedDir := filepath.Join(home, "seeded")
	if err := os.MkdirAll(filepath.Join(home, ".gemini"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".gemini", "projects.json"), []byte(`{"projects":{"`+seedDir+`":"seeded"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	targets := map[string]bool{
		sha256Hex(projectDir): true,
		sha256Hex(seedDir):    true,
		sha256Hex(heavyDir):   true, // should NOT be resolved (under node_modules)
	}
	resolved := resolveGeminiProjectPaths(targets)

	if resolved[sha256Hex(projectDir)] != projectDir {
		t.Fatalf("expected conclave resolved via walk, got %#v", resolved)
	}
	if resolved[sha256Hex(seedDir)] != seedDir {
		t.Fatalf("expected seeded project resolved from projects.json, got %#v", resolved)
	}
	if _, ok := resolved[sha256Hex(heavyDir)]; ok {
		t.Fatalf("node_modules tree should have been pruned, got %#v", resolved)
	}
}
