package app

import (
	"bufio"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const previewMaxRunes = 240

type SessionPreviewRequest struct {
	Agent       string `json:"agent"`
	SessionID   string `json:"sessionId"`
	ProjectPath string `json:"projectPath"`
}

type SessionPreviewResponse struct {
	SessionID             string `json:"sessionId"`
	Agent                 string `json:"agent"`
	Preview               string `json:"preview"`
	Timestamp             string `json:"timestamp"`
	SourcePath            string `json:"sourcePath"`
	ActiveDurationSeconds int64  `json:"activeDurationSeconds"`
	Originator            string `json:"originator"`
	ClientSource          string `json:"clientSource"`
	Model                 string `json:"model"`
	Provider              string `json:"provider"`
	ReasoningLevel        string `json:"reasoningLevel"`
	Cached                bool   `json:"cached"`
	Supported             bool   `json:"supported"`
	UnavailableHint       string `json:"unavailableHint"`
}

type SessionConversationResponse struct {
	SessionID             string                `json:"sessionId"`
	Agent                 string                `json:"agent"`
	ProjectPath           string                `json:"projectPath"`
	SourcePath            string                `json:"sourcePath"`
	ActiveDurationSeconds int64                 `json:"activeDurationSeconds"`
	Originator            string                `json:"originator"`
	ClientSource          string                `json:"clientSource"`
	Model                 string                `json:"model"`
	Provider              string                `json:"provider"`
	ReasoningLevel        string                `json:"reasoningLevel"`
	Messages              []ConversationMessage `json:"messages"`
	Supported             bool                  `json:"supported"`
	UnavailableHint       string                `json:"unavailableHint"`
}

type ConversationMessage struct {
	Role      string `json:"role"`
	Timestamp string `json:"timestamp"`
	Text      string `json:"text"`
}

type TranscriptMetadata struct {
	CWD            string
	Originator     string
	Source         string
	Model          string
	Provider       string
	ReasoningLevel string
}

func (a *App) GetSessionConversation(req SessionPreviewRequest) (SessionConversationResponse, error) {
	response := SessionConversationResponse{
		SessionID:   req.SessionID,
		Agent:       req.Agent,
		ProjectPath: req.ProjectPath,
		Supported:   true,
	}

	sourcePath, err := locateTranscript(req.Agent, req.SessionID, req.ProjectPath)
	if err != nil {
		response.Supported = false
		response.UnavailableHint = err.Error()
		return response, nil
	}

	transcriptMetadata := extractTranscriptMetadata(sourcePath)
	response.Originator = transcriptMetadata.Originator
	response.ClientSource = transcriptMetadata.Source
	response.Model = transcriptMetadata.Model
	response.Provider = transcriptMetadata.Provider
	response.ReasoningLevel = transcriptMetadata.ReasoningLevel
	messages, err := extractConversationMessages(sourcePath)
	response.SourcePath = sourcePath
	if err != nil {
		response.UnavailableHint = err.Error()
		return response, nil
	}
	response.Messages = messages
	response.ActiveDurationSeconds = activeDurationSeconds(messages)
	_ = writeCachedSessionTiming(sessionKey(req.Agent, req.SessionID), response.ActiveDurationSeconds, transcriptMetadata)
	return response, nil
}

func (a *App) GetSessionPreview(req SessionPreviewRequest) (SessionPreviewResponse, error) {
	response := SessionPreviewResponse{
		SessionID: req.SessionID,
		Agent:     req.Agent,
		Supported: true,
	}

	indexDBMutex.Lock()
	defer indexDBMutex.Unlock()

	db, _, err := openIndexDB()
	if err != nil {
		return response, err
	}
	defer db.Close()

	if err := migrateIndexDB(db); err != nil {
		return response, err
	}

	sessionKey := sessionKey(req.Agent, req.SessionID)
	if cached, ok, err := readCachedSessionPreview(db, sessionKey); err != nil {
		return response, err
	} else if ok {
		cached.SessionID = req.SessionID
		cached.Agent = req.Agent
		cached.Cached = true
		cached.Supported = true
		return cached, nil
	}

	sourcePath, err := locateTranscript(req.Agent, req.SessionID, req.ProjectPath)
	if err != nil {
		response.Supported = false
		response.UnavailableHint = err.Error()
		return response, nil
	}

	preview, timestamp, err := extractLastUserMessage(sourcePath)
	if err != nil {
		response.UnavailableHint = err.Error()
		response.SourcePath = sourcePath
		return response, nil
	}

	transcriptMetadata := extractTranscriptMetadata(sourcePath)
	messages, _ := extractConversationMessages(sourcePath)
	response.Preview = preview
	response.Timestamp = timestamp
	response.SourcePath = sourcePath
	response.ActiveDurationSeconds = activeDurationSeconds(messages)
	response.Originator = transcriptMetadata.Originator
	response.ClientSource = transcriptMetadata.Source
	response.Model = transcriptMetadata.Model
	response.Provider = transcriptMetadata.Provider
	response.ReasoningLevel = transcriptMetadata.ReasoningLevel
	if err := writeCachedSessionPreview(db, sessionKey, response); err != nil {
		return response, err
	}

	return response, nil
}

func readCachedSessionPreview(db *sql.DB, sessionKey string) (SessionPreviewResponse, bool, error) {
	response := SessionPreviewResponse{}
	err := db.QueryRow(`SELECT
		last_user_message_preview,
		last_user_message_at,
		message_source_path,
		active_duration_seconds,
		originator,
		client_source,
		model,
		provider,
		reasoning_level
	FROM project_sessions
	WHERE session_key = ?`, sessionKey).Scan(&response.Preview, &response.Timestamp, &response.SourcePath, &response.ActiveDurationSeconds, &response.Originator, &response.ClientSource, &response.Model, &response.Provider, &response.ReasoningLevel)
	if errors.Is(err, sql.ErrNoRows) {
		return response, false, nil
	}
	if err != nil {
		return response, false, err
	}
	return response, response.Preview != "", nil
}

func writeCachedSessionPreview(db *sql.DB, sessionKey string, response SessionPreviewResponse) error {
	_, err := db.Exec(`UPDATE project_sessions
	SET last_user_message_preview = ?, last_user_message_at = ?, message_source_path = ?, active_duration_seconds = ?, originator = ?, client_source = ?, model = ?, provider = ?, reasoning_level = ?
	WHERE session_key = ?`, response.Preview, response.Timestamp, response.SourcePath, response.ActiveDurationSeconds, response.Originator, response.ClientSource, response.Model, response.Provider, response.ReasoningLevel, sessionKey)
	return err
}

func writeCachedSessionTiming(sessionKey string, activeDurationSeconds int64, metadata TranscriptMetadata) error {
	indexDBMutex.Lock()
	defer indexDBMutex.Unlock()

	db, _, err := openIndexDB()
	if err != nil {
		return err
	}
	defer db.Close()
	if err := migrateIndexDB(db); err != nil {
		return err
	}
	_, err = db.Exec(`UPDATE project_sessions SET active_duration_seconds = ?, originator = ?, client_source = ?, model = ?, provider = ?, reasoning_level = ? WHERE session_key = ?`, activeDurationSeconds, metadata.Originator, metadata.Source, metadata.Model, metadata.Provider, metadata.ReasoningLevel, sessionKey)
	return err
}

func sessionKey(agent string, sessionID string) string {
	if agent == "" {
		agent = "all"
	}
	return agent + ":" + sessionID
}

func locateTranscript(agent string, sessionID string, projectPath string) (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	switch agent {
	case "pi":
		return locatePiTranscript(homeDir, sessionID, projectPath)
	case "codex":
		return locateCodexTranscript(homeDir, sessionID)
	case "claude":
		return locateClaudeTranscript(homeDir, sessionID)
	case "opencode":
		return locateOpenCodeTranscript(homeDir, sessionID)
	case "gemini":
		return locateGeminiTranscript(homeDir, sessionID)
	default:
		return "", errors.New("preview is only supported for claude, codex, gemini, opencode, and pi sessions")
	}
}

func locatePiTranscript(homeDir string, sessionID string, projectPath string) (string, error) {
	if projectPath != "" && projectPath != "(unknown)" {
		projectDir := filepath.Join(homeDir, ".pi", "agent", "sessions", projectPath)
		if match, ok := findFileContaining(projectDir, sessionID); ok {
			return match, nil
		}
	}
	return findJSONLByName(filepath.Join(homeDir, ".pi", "agent", "sessions"), sessionID)
}

func locateCodexTranscript(homeDir string, sessionID string) (string, error) {
	if strings.Contains(sessionID, "/") {
		path := filepath.Join(homeDir, ".codex", "sessions", sessionID+".jsonl")
		if fileExists(path) {
			return path, nil
		}
		archivedPath := filepath.Join(homeDir, ".codex", "archived_sessions", filepath.Base(sessionID)+".jsonl")
		if fileExists(archivedPath) {
			return archivedPath, nil
		}
	}
	return findJSONLByName(filepath.Join(homeDir, ".codex", "sessions"), sessionID)
}

func locateClaudeTranscript(homeDir string, sessionID string) (string, error) {
	transcriptsDir := filepath.Join(homeDir, ".claude", "transcripts")
	path := filepath.Join(transcriptsDir, sessionID+".jsonl")
	if fileExists(path) {
		return path, nil
	}
	if match, err := findJSONLByName(transcriptsDir, sessionID); err == nil {
		return match, nil
	}
	return findJSONLByName(filepath.Join(homeDir, ".claude", "projects"), sessionID)
}

func locateOpenCodeTranscript(homeDir string, sessionID string) (string, error) {
	dbPath := filepath.Join(homeDir, ".local", "share", "opencode", "opencode.db")
	if !fileExists(dbPath) {
		return "", errors.New("could not find opencode database")
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	transcriptDir := filepath.Join(cacheDir, "ccusage-ui", "opencode-transcripts")
	if err := os.MkdirAll(transcriptDir, 0o755); err != nil {
		return "", err
	}
	transcriptPath := filepath.Join(transcriptDir, sessionID+".jsonl")

	if err := exportOpenCodeSession(dbPath, sessionID, transcriptPath); err != nil {
		return "", err
	}
	return transcriptPath, nil
}

func exportOpenCodeSession(dbPath string, sessionID string, transcriptPath string) error {
	db, err := sql.Open("sqlite", dbPath+"?mode=ro&_pragma=busy_timeout(5000)")
	if err != nil {
		return err
	}
	defer db.Close()

	rows, err := db.Query(`SELECT
		m.id,
		m.time_created,
		m.data,
		s.directory
	FROM message m
	JOIN session s ON s.id = m.session_id
	WHERE m.session_id = ?
	ORDER BY m.time_created, m.id`, sessionID)
	if err != nil {
		return err
	}
	defer rows.Close()

	var lines []string
	for rows.Next() {
		var messageID string
		var createdMillis int64
		var rawMessage string
		var sessionDirectory string
		if err := rows.Scan(&messageID, &createdMillis, &rawMessage, &sessionDirectory); err != nil {
			return err
		}

		message := map[string]any{}
		if err := json.Unmarshal([]byte(rawMessage), &message); err != nil {
			continue
		}
		role := stringValue(message["role"])
		if role != "user" && role != "assistant" {
			continue
		}

		text, err := readOpenCodeMessageText(db, sessionID, messageID)
		if err != nil {
			return err
		}
		text = strings.TrimSpace(text)
		if text == "" {
			continue
		}

		line := map[string]any{
			"type":      role,
			"role":      role,
			"timestamp": time.UnixMilli(createdMillis).Format(time.RFC3339Nano),
			"content":   text,
			"cwd":       firstNonEmptyString(openCodeMessageCWD(message), sessionDirectory),
			"source":    "opencode",
			"model":     stringValue(message["modelID"]),
			"provider":  stringValue(message["providerID"]),
		}
		encoded, err := json.Marshal(line)
		if err != nil {
			return err
		}
		lines = append(lines, string(encoded))
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if len(lines) == 0 {
		return errors.New("no opencode conversation messages found")
	}

	return os.WriteFile(transcriptPath, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

func readOpenCodeMessageText(db *sql.DB, sessionID string, messageID string) (string, error) {
	rows, err := db.Query(`SELECT data FROM part WHERE session_id = ? AND message_id = ? ORDER BY time_created, id`, sessionID, messageID)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	parts := []string{}
	for rows.Next() {
		var rawPart string
		if err := rows.Scan(&rawPart); err != nil {
			return "", err
		}
		part := map[string]any{}
		if err := json.Unmarshal([]byte(rawPart), &part); err != nil {
			continue
		}
		if stringValue(part["type"]) != "text" {
			continue
		}
		if text := strings.TrimSpace(stringValue(part["text"])); text != "" {
			parts = append(parts, text)
		}
	}
	if err := rows.Err(); err != nil {
		return "", err
	}
	return strings.Join(parts, "\n\n"), nil
}

func openCodeMessageCWD(message map[string]any) string {
	path, ok := message["path"].(map[string]any)
	if !ok {
		return ""
	}
	return stringValue(path["cwd"])
}

func locateGeminiTranscript(homeDir string, sessionID string) (string, error) {
	sourcePath, err := findGeminiChatFile(homeDir, sessionID)
	if err != nil {
		return "", err
	}

	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	transcriptDir := filepath.Join(cacheDir, "ccusage-ui", "gemini-transcripts")
	if err := os.MkdirAll(transcriptDir, 0o755); err != nil {
		return "", err
	}
	transcriptPath := filepath.Join(transcriptDir, sessionID+".jsonl")

	if err := exportGeminiSession(sourcePath, transcriptPath); err != nil {
		return "", err
	}
	return transcriptPath, nil
}

// findGeminiChatFile locates the Gemini CLI chat export for a session. Files are
// stored under ~/.gemini/tmp/<projectHash>/chats/session-<timestamp>-<id>.json,
// where <id> is the first 8 hex chars of the session UUID.
func findGeminiChatFile(homeDir string, sessionID string) (string, error) {
	fragment := geminiSessionFragment(sessionID)
	root := filepath.Join(homeDir, ".gemini", "tmp")
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || found != "" {
			return nil
		}
		name := entry.Name()
		if strings.HasPrefix(name, "session-") && strings.HasSuffix(name, ".json") && strings.Contains(name, fragment) {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", errors.New("could not find gemini transcript file")
	}
	return found, nil
}

func geminiSessionFragment(sessionID string) string {
	cleaned := strings.ReplaceAll(sessionID, "-", "")
	if len(cleaned) >= 8 {
		return cleaned[:8]
	}
	return sessionID
}

// exportGeminiSession converts a Gemini CLI chat export (a single JSON object)
// into the JSONL shape the transcript readers expect, mapping the "gemini"
// assistant message type to role "assistant".
func exportGeminiSession(sourcePath string, transcriptPath string) error {
	data, err := os.ReadFile(sourcePath)
	if err != nil {
		return err
	}
	var session struct {
		Messages []struct {
			Timestamp string `json:"timestamp"`
			Type      string `json:"type"`
			Role      string `json:"role"`
			Content   string `json:"content"`
		} `json:"messages"`
	}
	if err := json.Unmarshal(data, &session); err != nil {
		return err
	}

	lines := make([]string, 0, len(session.Messages))
	for _, message := range session.Messages {
		role := strings.TrimSpace(message.Role)
		if role == "" {
			if message.Type == "user" {
				role = "user"
			} else {
				role = "assistant"
			}
		}
		if role != "user" && role != "assistant" {
			continue
		}
		text := strings.TrimSpace(message.Content)
		if text == "" {
			continue
		}
		encoded, err := json.Marshal(map[string]any{
			"type":      role,
			"role":      role,
			"timestamp": message.Timestamp,
			"content":   text,
			"source":    "gemini",
			"provider":  "google",
		})
		if err != nil {
			return err
		}
		lines = append(lines, string(encoded))
	}
	if len(lines) == 0 {
		return errors.New("no gemini conversation messages found")
	}
	return os.WriteFile(transcriptPath, []byte(strings.Join(lines, "\n")+"\n"), 0o600)
}

// geminiSessionMeta captures what the indexer needs from a Gemini chat export
// that ccusage does not provide: a real last-activity timestamp and the project
// hash used to recover the working directory.
type geminiSessionMeta struct {
	lastActivity string
	projectHash  string
}

// readGeminiSessionMeta scans Gemini CLI chat exports once and maps session
// UUIDs to their lastUpdated timestamp and project hash. ccusage does not expose
// lastActivity or projectPath for Gemini sessions, so the transcript is the only
// source. The project hash is the ~/.gemini/tmp/<hash>/ directory name.
func readGeminiSessionMeta() map[string]geminiSessionMeta {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return map[string]geminiSessionMeta{}
	}
	root := filepath.Join(homeDir, ".gemini", "tmp")
	out := map[string]geminiSessionMeta{}
	_ = filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() {
			return nil
		}
		name := entry.Name()
		if !strings.HasPrefix(name, "session-") || !strings.HasSuffix(name, ".json") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return nil
		}
		var session struct {
			SessionID   string `json:"sessionId"`
			LastUpdated string `json:"lastUpdated"`
		}
		if err := json.Unmarshal(data, &session); err != nil {
			return nil
		}
		if session.SessionID == "" {
			return nil
		}
		// path = ~/.gemini/tmp/<hash>/chats/<file> -> <hash> is two dirs up.
		projectHash := filepath.Base(filepath.Dir(filepath.Dir(path)))
		out[session.SessionID] = geminiSessionMeta{lastActivity: session.LastUpdated, projectHash: projectHash}
		return nil
	})
	return out
}

// resolveGeminiProjectPaths maps Gemini project hashes back to their absolute
// path. Gemini stores projects under ~/.gemini/tmp/<SHA-256(path)>/, so the hash
// is one-way and must be resolved by hashing candidate paths. Seeds come from
// Gemini's own config (projects.json, trustedFolders.json); the remainder are
// discovered by walking the home directory (bounded depth, heavy trees pruned).
func resolveGeminiProjectPaths(targets map[string]bool) map[string]string {
	out := map[string]string{}
	if len(targets) == 0 {
		return out
	}
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return out
	}

	consider := func(path string) {
		if path == "" {
			return
		}
		hash := sha256Hex(path)
		if targets[hash] && out[hash] == "" {
			out[hash] = path
		}
	}

	// Direct hits from Gemini config (paths may sit outside the walked depth).
	for _, path := range geminiKnownProjectPaths(homeDir) {
		consider(path)
	}
	if len(out) >= len(targets) {
		return out
	}

	// Heavy/system/package trees that must not be descended into. The walk is
	// also depth-capped so we consider project roots but not their source trees.
	skip := map[string]bool{
		"Library": true, ".Trash": true, "Caches": true, ".cache": true,
		"Pictures": true, "Music": true, "Movies": true, "Downloads": true,
		"Applications": true, "Public": true, "Parallels": true, "OneDrive": true,
		"node_modules": true, ".git": true, ".svn": true, ".hg": true,
		".npm": true, ".pnpm": true, ".yarn": true, ".cargo": true, ".rustup": true,
		".docker": true, ".cursor": true, ".vscode": true, ".idea": true,
		".gradle": true, ".m2": true, ".terraform": true, ".serverless": true,
		"miniconda": true, "anaconda": true, ".conda": true, ".pyenv": true,
		"venv": true, ".venv": true, "__pycache__": true, ".tox": true,
		".mypy_cache": true, ".pytest_cache": true, "go": true, ".bun": true, ".deno": true,
		"dist": true, "build": true, "target": true, "out": true,
		".next": true, ".turbo": true, ".nuxt": true, ".svelte-kit": true,
		"coverage": true, ".nyc_output": true,
	}
	separator := string(filepath.Separator)
	const maxDepth = 4
	_ = filepath.WalkDir(homeDir, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry == nil {
			return nil
		}
		if len(out) >= len(targets) {
			return filepath.SkipAll
		}
		if path == homeDir || !entry.IsDir() {
			return nil
		}
		if skip[entry.Name()] {
			return filepath.SkipDir
		}
		consider(path)
		depth := strings.Count(strings.TrimPrefix(path, homeDir), separator)
		if depth >= maxDepth {
			return filepath.SkipDir
		}
		return nil
	})
	return out
}

// geminiKnownProjectPaths returns project paths Gemini records in its config.
// projects.json is {"projects": {path: name}}; trustedFolders.json is
// {path: "TRUST_FOLDER"}. These are exact seeds but usually incomplete (only
// named/trusted projects), so the filesystem walk fills in the rest.
func geminiKnownProjectPaths(homeDir string) []string {
	var paths []string
	if data, err := os.ReadFile(filepath.Join(homeDir, ".gemini", "projects.json")); err == nil {
		var wrapper struct {
			Projects map[string]any `json:"projects"`
		}
		if json.Unmarshal(data, &wrapper) == nil {
			for path := range wrapper.Projects {
				paths = append(paths, path)
			}
		}
	}
	if data, err := os.ReadFile(filepath.Join(homeDir, ".gemini", "trustedFolders.json")); err == nil {
		var flat map[string]any
		if json.Unmarshal(data, &flat) == nil {
			for path := range flat {
				paths = append(paths, path)
			}
		}
	}
	return paths
}

func sha256Hex(value string) string {
	sum := sha256.Sum256([]byte(value))
	return hex.EncodeToString(sum[:])
}

func findFileContaining(dir string, fragment string) (string, bool) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return "", false
	}
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".jsonl") && strings.Contains(name, fragment) {
			return filepath.Join(dir, name), true
		}
	}
	return "", false
}

func findJSONLByName(root string, fragment string) (string, error) {
	var found string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil || entry.IsDir() || found != "" {
			return nil
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".jsonl") && strings.Contains(name, fragment) {
			found = path
			return filepath.SkipAll
		}
		return nil
	})
	if err != nil {
		return "", err
	}
	if found == "" {
		return "", errors.New("could not find transcript file")
	}
	return found, nil
}

func fileExists(path string) bool {
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func extractTranscriptCWD(path string) string {
	return extractTranscriptMetadata(path).CWD
}

func extractTranscriptMetadata(path string) TranscriptMetadata {
	file, err := os.Open(path)
	if err != nil {
		return TranscriptMetadata{}
	}
	defer file.Close()

	metadata := TranscriptMetadata{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)
	for scanner.Scan() {
		var payload map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &payload); err != nil {
			continue
		}
		metadata.CWD = firstNonEmptyString(metadata.CWD, stringValue(payload["cwd"]))
		metadata.Originator = firstNonEmptyString(metadata.Originator, stringValue(payload["originator"]))
		metadata.Source = firstNonEmptyString(metadata.Source, stringValue(payload["source"]))
		if payload["type"] == "model_change" {
			metadata.Provider = firstNonEmptyString(metadata.Provider, stringValue(payload["provider"]))
			metadata.Model = firstNonEmptyString(metadata.Model, stringValue(payload["modelId"]))
		}
		if payload["type"] == "thinking_level_change" {
			metadata.ReasoningLevel = firstNonEmptyString(metadata.ReasoningLevel, stringValue(payload["thinkingLevel"]))
		}
		if nested, ok := payload["payload"].(map[string]any); ok {
			metadata.CWD = firstNonEmptyString(metadata.CWD, stringValue(nested["cwd"]))
			metadata.Originator = firstNonEmptyString(metadata.Originator, stringValue(nested["originator"]))
			metadata.Source = firstNonEmptyString(metadata.Source, stringValue(nested["source"]))
			metadata.Provider = firstNonEmptyString(metadata.Provider, stringValue(nested["model_provider"]))
			metadata.Model = firstNonEmptyString(metadata.Model, stringValue(nested["model"]))
			if collaborationMode, ok := nested["collaboration_mode"].(map[string]any); ok {
				if settings, ok := collaborationMode["settings"].(map[string]any); ok {
					metadata.Model = firstNonEmptyString(metadata.Model, stringValue(settings["model"]))
					metadata.ReasoningLevel = firstNonEmptyString(metadata.ReasoningLevel, stringValue(settings["reasoning_effort"]))
				}
			}
		}
		if metadata.CWD != "" && metadata.Originator != "" && metadata.Source != "" && metadata.Model != "" && metadata.ReasoningLevel != "" {
			return metadata
		}
	}
	return metadata
}

func extractConversationMessages(path string) ([]ConversationMessage, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)

	messages := []ConversationMessage{}
	for scanner.Scan() {
		message := extractConversationMessageFromJSONLine(scanner.Bytes())
		if message.Text == "" {
			continue
		}
		messages = append(messages, message)
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	if len(messages) == 0 {
		return nil, errors.New("no conversation messages found in transcript")
	}
	return messages, nil
}

func extractConversationMessageFromJSONLine(line []byte) ConversationMessage {
	var payload map[string]any
	if err := json.Unmarshal(line, &payload); err != nil {
		return ConversationMessage{}
	}

	if payload["type"] == "response_item" {
		if nested, ok := payload["payload"].(map[string]any); ok {
			return conversationMessageFromMap(nested, stringValue(payload["timestamp"]))
		}
	}

	if payload["type"] == "message" || payload["type"] == "user" {
		if nested, ok := payload["message"].(map[string]any); ok {
			if payload["type"] == "user" && stringValue(nested["role"]) == "" {
				nested["role"] = "user"
			}
			return conversationMessageFromMap(nested, firstNonEmptyString(stringValue(payload["timestamp"]), stringValue(nested["timestamp"])))
		}
	}

	if payloadType := stringValue(payload["type"]); payloadType == "user" || payloadType == "assistant" {
		if stringValue(payload["role"]) == "" {
			payload["role"] = payloadType
		}
	}

	return conversationMessageFromMap(payload, stringValue(payload["timestamp"]))
}

func conversationMessageFromMap(message map[string]any, timestamp string) ConversationMessage {
	role := stringValue(message["role"])
	if role != "user" && role != "assistant" {
		return ConversationMessage{}
	}

	text := messageContentText(message["content"])
	if text == "" {
		text = firstNonEmptyString(stringValue(message["text"]), stringValue(message["lastPrompt"]))
	}
	text = strings.TrimSpace(text)
	if text == "" {
		return ConversationMessage{}
	}

	return ConversationMessage{Role: role, Timestamp: timestamp, Text: text}
}

func messageContentText(content any) string {
	switch typed := content.(type) {
	case string:
		return typed
	case []any:
		parts := make([]string, 0, len(typed))
		for _, item := range typed {
			itemMap, ok := item.(map[string]any)
			if !ok {
				continue
			}
			itemType := stringValue(itemMap["type"])
			if itemType == "tool_result" || itemType == "toolResult" || itemType == "function_call_output" || itemType == "thinking" {
				continue
			}
			if text := firstNonEmptyString(stringValue(itemMap["text"]), stringValue(itemMap["content"])); text != "" {
				parts = append(parts, text)
			}
		}
		return strings.Join(parts, "\n\n")
	default:
		return ""
	}
}

func activeDurationSeconds(messages []ConversationMessage) int64 {
	var active time.Duration
	var turnStart time.Time
	var lastAssistant time.Time
	inTurn := false

	flushTurn := func() {
		if inTurn && !turnStart.IsZero() && lastAssistant.After(turnStart) {
			active += lastAssistant.Sub(turnStart)
		}
		turnStart = time.Time{}
		lastAssistant = time.Time{}
		inTurn = false
	}

	for _, message := range messages {
		timestamp, ok := parseTranscriptTime(message.Timestamp)
		if !ok {
			continue
		}
		switch message.Role {
		case "user":
			flushTurn()
			turnStart = timestamp
			inTurn = true
		case "assistant":
			if inTurn && timestamp.After(lastAssistant) {
				lastAssistant = timestamp
			}
		}
	}
	flushTurn()
	return int64(active.Seconds())
}

func parseTranscriptTime(value string) (time.Time, bool) {
	if value == "" {
		return time.Time{}, false
	}
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed, true
	}
	if parsed, err := time.Parse(time.RFC3339, value); err == nil {
		return parsed, true
	}
	return time.Time{}, false
}

func extractLastUserMessage(path string) (string, string, error) {
	file, err := os.Open(path)
	if err != nil {
		return "", "", err
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 0, 1024*1024), 16*1024*1024)

	lastText := ""
	lastTimestamp := ""
	for scanner.Scan() {
		text, timestamp := extractUserMessageFromJSONLine(scanner.Bytes())
		if text == "" {
			continue
		}
		lastText = normalizePreview(text)
		lastTimestamp = timestamp
	}
	if err := scanner.Err(); err != nil {
		return "", "", err
	}
	if lastText == "" {
		return "", "", errors.New("no user message found in transcript")
	}
	return lastText, lastTimestamp, nil
}

func extractUserMessageFromJSONLine(line []byte) (string, string) {
	var payload map[string]any
	if err := json.Unmarshal(line, &payload); err != nil {
		return "", ""
	}

	if payload["type"] == "last-prompt" {
		if text, ok := payload["lastPrompt"].(string); ok {
			return text, stringValue(payload["timestamp"])
		}
	}

	if payload["type"] == "response_item" {
		if nested, ok := payload["payload"].(map[string]any); ok {
			return messageText(nested, stringValue(payload["timestamp"]))
		}
	}

	if payload["type"] == "message" {
		if nested, ok := payload["message"].(map[string]any); ok {
			return messageText(nested, firstNonEmptyString(stringValue(payload["timestamp"]), stringValue(nested["timestamp"])))
		}
	}

	if payload["type"] == "user" {
		if nested, ok := payload["message"].(map[string]any); ok {
			if stringValue(nested["role"]) == "" {
				nested["role"] = "user"
			}
			return messageText(nested, stringValue(payload["timestamp"]))
		}
		if stringValue(payload["role"]) == "" {
			payload["role"] = "user"
		}
	}

	return messageText(payload, stringValue(payload["timestamp"]))
}

func messageText(message map[string]any, timestamp string) (string, string) {
	if message["role"] != "user" {
		return "", ""
	}

	text := messageContentText(message["content"])
	if text == "" {
		return "", ""
	}
	return text, timestamp
}

func normalizePreview(text string) string {
	text = strings.TrimSpace(strings.Join(strings.Fields(text), " "))
	runes := []rune(text)
	if len(runes) <= previewMaxRunes {
		return text
	}
	return string(runes[:previewMaxRunes-1]) + "..."
}

func stringValue(value any) string {
	switch typed := value.(type) {
	case string:
		return typed
	case float64:
		return time.UnixMilli(int64(typed)).Format(time.RFC3339)
	default:
		return ""
	}
}

func firstNonEmptyString(values ...string) string {
	for _, value := range values {
		if value != "" {
			return value
		}
	}
	return ""
}
