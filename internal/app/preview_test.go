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

	if len(messages) != 3 {
		t.Fatalf("expected 3 messages, got %d", len(messages))
	}
	if messages[0].Role != "user" || messages[0].Text != "hello claude" {
		t.Fatalf("unexpected user message: %#v", messages[0])
	}
	if messages[1].Role != "event" || messages[1].Type != "tool_use" || !messages[1].HiddenByDefault {
		t.Fatalf("unexpected trace event: %#v", messages[1])
	}
	if messages[2].Role != "assistant" || messages[2].Text != "hello user" {
		t.Fatalf("unexpected assistant message: %#v", messages[2])
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

func TestResolveGeminiNamedProjects(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	if err := os.MkdirAll(filepath.Join(home, ".gemini"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(home, ".gemini", "projects.json"), []byte(`{"projects":{"`+filepath.Join(home, "kaden")+`":"kaden"}}`), 0o600); err != nil {
		t.Fatal(err)
	}

	// "kaden" is a name target (Gemini stores named projects under tmp/<name>/).
	resolved := resolveGeminiProjectPaths(map[string]bool{"kaden": true})
	if resolved["kaden"] != filepath.Join(home, "kaden") {
		t.Fatalf("expected named project resolved via projects.json, got %#v", resolved)
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

func TestExtractConversationMessagesFromQwenJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	// Qwen Code wraps messages under "message", carries text in "parts", and
	// labels assistant turns with role "model".
	content := `{"type":"user","timestamp":"2026-01-03T12:02:10.906Z","message":{"role":"user","parts":[{"text":"explain this repo"}]}}
{"type":"assistant","timestamp":"2026-01-03T12:02:14.794Z","model":"qwen3","message":{"role":"model","parts":[{"text":"it is a dotfiles repo"}]}}
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
	if messages[0].Role != "user" || messages[0].Text != "explain this repo" {
		t.Fatalf("unexpected user message: %#v", messages[0])
	}
	if messages[1].Role != "assistant" || messages[1].Text != "it is a dotfiles repo" {
		t.Fatalf("expected qwen model role mapped to assistant with parts text: %#v", messages[1])
	}
}

func TestLocateQwenTranscript(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	chats := filepath.Join(home, ".qwen", "projects", "-Users-example-app", "chats")
	if err := os.MkdirAll(chats, 0o755); err != nil {
		t.Fatal(err)
	}
	const sid = "07b4abd0-0f15-4069-8639-6aeaaf3fc7eb"
	if err := os.WriteFile(filepath.Join(chats, sid+".jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := locateQwenTranscript(home, sid)
	if err != nil {
		t.Fatalf("locateQwenTranscript failed: %v", err)
	}
	if filepath.Base(got) != sid+".jsonl" {
		t.Fatalf("expected session transcript, got %s", got)
	}
}

func TestLocateDroidTranscript(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sessions := filepath.Join(home, ".factory", "sessions", "-Users-example-app")
	if err := os.MkdirAll(sessions, 0o755); err != nil {
		t.Fatal(err)
	}
	const sid = "71c73c67-90f7-49fd-af1e-2eaa17f1a11c"
	if err := os.WriteFile(filepath.Join(sessions, sid+".jsonl"), []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	// Sidecar settings file must not be matched.
	if err := os.WriteFile(filepath.Join(sessions, sid+".settings.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	got, err := locateDroidTranscript(home, sid)
	if err != nil {
		t.Fatalf("locateDroidTranscript failed: %v", err)
	}
	if filepath.Base(got) != sid+".jsonl" {
		t.Fatalf("expected session transcript, got %s", got)
	}
}

func TestLastTimestampInJSONL(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "session.jsonl")
	content := `{"type":"session_start","timestamp":"2026-02-20T06:29:42.000Z"}
{"type":"message","timestamp":"2026-02-20T06:29:58.526Z","message":{"role":"assistant"}}
{"type":"message","timestamp":"2026-02-20T06:29:50.000Z","message":{"role":"user"}}
`
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	if ts := lastTimestampInJSONL(path); ts != "2026-02-20T06:29:58.526Z" {
		t.Fatalf("expected latest timestamp, got %q", ts)
	}
}

func TestReadDroidSessionMeta(t *testing.T) {
	home := t.TempDir()
	t.Setenv("HOME", home)

	sessions := filepath.Join(home, ".factory", "sessions", "-Users-example-app")
	if err := os.MkdirAll(sessions, 0o755); err != nil {
		t.Fatal(err)
	}
	const sid = "71c73c67-90f7-49fd-af1e-2eaa17f1a11c"
	content := `{"type":"message","timestamp":"2026-02-20T06:29:50.000Z","message":{"role":"user"}}
{"type":"message","timestamp":"2026-02-20T06:29:58.526Z","message":{"role":"assistant"}}
`
	if err := os.WriteFile(filepath.Join(sessions, sid+".jsonl"), []byte(content), 0o600); err != nil {
		t.Fatal(err)
	}
	// Sidecar settings file must be ignored.
	if err := os.WriteFile(filepath.Join(sessions, sid+".settings.json"), []byte("{}"), 0o600); err != nil {
		t.Fatal(err)
	}

	meta := readDroidSessionMeta()
	if ts, ok := meta[sid]; !ok || ts != "2026-02-20T06:29:58.526Z" {
		t.Fatalf("expected droid last activity mapped, got %#v", meta)
	}
}
