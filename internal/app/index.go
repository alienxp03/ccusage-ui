package app

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

var indexDBMutex sync.Mutex

type IndexRequest struct {
	Source  string `json:"source"`
	Since   string `json:"since"`
	Until   string `json:"until"`
	Offline bool   `json:"offline"`
	NoCost  bool   `json:"noCost"`
}

// indexFilter narrows the cached session index by source (agent) and a
// last-activity window at read time. The index is scanned once across all
// sources/all time, so views can be filtered cheaply without re-scanning.
type indexFilter struct {
	source string // "all" (or "") disables the source filter
	since  string // inclusive lower bound on last_activity (YYYY-MM-DD); "" disables
	until  string // exclusive upper bound on last_activity (next-day YYYY-MM-DD); "" disables
}

func newIndexFilter(req IndexRequest) indexFilter {
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "all"
	}
	return indexFilter{source: source, since: strings.TrimSpace(req.Since), until: strings.TrimSpace(req.Until)}
}

// conditions builds the AND-joined WHERE fragments that scope project_sessions
// rows (qualified by alias) to the filter. alias is the SQL qualifier for the
// project_sessions table in the surrounding query (e.g. "project_sessions" or
// "ps"). Returns "" with nil args when no filter applies.
//
// last_activity is stored as an ISO-8601 timestamp (e.g. "2026-06-25T10:00:00Z")
// while since/until are YYYY-MM-DD, so a lexicographic comparison matches the
// calendar window. until is the exclusive next day (mirrors ccusage --until).
func (f indexFilter) conditions(alias string) (string, []any) {
	var conds []string
	var args []any
	if f.source != "" && f.source != "all" {
		conds = append(conds, alias+".agent = ?")
		args = append(args, f.source)
	}
	if f.since != "" {
		conds = append(conds, alias+".last_activity >= ?")
		args = append(args, f.since)
	}
	if f.until != "" {
		conds = append(conds, alias+".last_activity < ?")
		args = append(args, f.until)
	}
	if len(conds) == 0 {
		return "", nil
	}
	return strings.Join(conds, " AND "), args
}

type ProjectIndexResponse struct {
	Projects    []ProjectSummary `json:"projects"`
	AgentGroups []IndexGroup     `json:"agentGroups"`
	ModelGroups []IndexGroup     `json:"modelGroups"`
	Database    string           `json:"database"`
	LastIndexed string           `json:"lastIndexed"`
	Runner      RunnerInfo       `json:"runner"`
	Command     []string         `json:"command"`
	Generated   string           `json:"generated"`
}

type ProjectSummary struct {
	ProjectPath         string           `json:"projectPath"`
	ProjectName         string           `json:"projectName"`
	PhysicalPaths       []string         `json:"physicalPaths"`
	PathExists          bool             `json:"pathExists"`
	GroupingRule        string           `json:"groupingRule"`
	Agents              []string         `json:"agents"`
	SessionCount        int64            `json:"sessionCount"`
	LastActivity        string           `json:"lastActivity"`
	InputTokens         int64            `json:"inputTokens"`
	OutputTokens        int64            `json:"outputTokens"`
	CacheCreationTokens int64            `json:"cacheCreationTokens"`
	CacheReadTokens     int64            `json:"cacheReadTokens"`
	TotalTokens         int64            `json:"totalTokens"`
	TotalCost           float64          `json:"totalCost"`
	ModelBreakdowns     []ModelBreakdown `json:"modelBreakdowns"`
	Activity            ActivitySummary  `json:"activity"`
	RecentSessions      []IndexedSession `json:"recentSessions"`
}

type IndexGroup struct {
	Name                string           `json:"name"`
	GroupBy             string           `json:"groupBy"`
	ProjectCount        int64            `json:"projectCount"`
	SessionCount        int64            `json:"sessionCount"`
	LastActivity        string           `json:"lastActivity"`
	InputTokens         int64            `json:"inputTokens"`
	OutputTokens        int64            `json:"outputTokens"`
	CacheCreationTokens int64            `json:"cacheCreationTokens"`
	CacheReadTokens     int64            `json:"cacheReadTokens"`
	TotalTokens         int64            `json:"totalTokens"`
	TotalCost           float64          `json:"totalCost"`
	Agents              []string         `json:"agents"`
	ModelBreakdowns     []ModelBreakdown `json:"modelBreakdowns"`
}

type IndexedSession struct {
	SessionID             string           `json:"sessionId"`
	Agent                 string           `json:"agent"`
	ProjectPath           string           `json:"projectPath"`
	ProjectName           string           `json:"projectName"`
	LastActivity          string           `json:"lastActivity"`
	InputTokens           int64            `json:"inputTokens"`
	OutputTokens          int64            `json:"outputTokens"`
	CacheCreationTokens   int64            `json:"cacheCreationTokens"`
	CacheReadTokens       int64            `json:"cacheReadTokens"`
	TotalTokens           int64            `json:"totalTokens"`
	TotalCost             float64          `json:"totalCost"`
	ModelBreakdowns       []ModelBreakdown `json:"modelBreakdowns"`
	LastUserMessage       string           `json:"lastUserMessage"`
	LastUserMessageAt     string           `json:"lastUserMessageAt"`
	MessageSourcePath     string           `json:"messageSourcePath"`
	ActiveDurationSeconds int64            `json:"activeDurationSeconds"`
	Originator            string           `json:"originator"`
	ClientSource          string           `json:"clientSource"`
	Model                 string           `json:"model"`
	Provider              string           `json:"provider"`
	ReasoningLevel        string           `json:"reasoningLevel"`
	Activity              ActivitySummary  `json:"activity"`
}

func (a *App) RefreshProjectIndex(req IndexRequest) (ProjectIndexResponse, error) {
	// Always scan the full history across all sources so source/date filters can
	// be applied at read time. Only offline/no-cost (cost computation) are honored.
	broadScan := IndexRequest{Offline: req.Offline, NoCost: req.NoCost}
	rows, runner, command, err := a.loadSessionRows(broadScan)
	if err != nil {
		return ProjectIndexResponse{}, err
	}

	indexDBMutex.Lock()
	defer indexDBMutex.Unlock()

	db, dbPath, err := openIndexDB()
	if err != nil {
		return ProjectIndexResponse{}, err
	}
	defer db.Close()

	if err := migrateIndexDB(db); err != nil {
		return ProjectIndexResponse{}, err
	}

	indexedAt := time.Now().Format(time.RFC3339)
	if err := replaceIndexedSessions(db, rows, indexedAt); err != nil {
		return ProjectIndexResponse{}, err
	}

	response, err := readProjectIndex(db, dbPath, newIndexFilter(req))
	if err != nil {
		return ProjectIndexResponse{}, err
	}
	response.Runner = runner
	response.Command = command
	response.Generated = time.Now().Format(time.RFC3339)
	return response, nil
}

func (a *App) GetProjectIndex(req IndexRequest) (ProjectIndexResponse, error) {
	indexDBMutex.Lock()
	defer indexDBMutex.Unlock()

	db, dbPath, err := openIndexDB()
	if err != nil {
		return ProjectIndexResponse{}, err
	}
	defer db.Close()

	if err := migrateIndexDB(db); err != nil {
		return ProjectIndexResponse{}, err
	}

	response, err := readProjectIndex(db, dbPath, newIndexFilter(req))
	if err != nil {
		return ProjectIndexResponse{}, err
	}
	response.Generated = time.Now().Format(time.RFC3339)
	response.Runner = detectRunner()
	return response, nil
}

func (a *App) loadSessionRows(req IndexRequest) ([]ReportRow, RunnerInfo, []string, error) {
	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "all"
	}
	if !slices.Contains(sourceNames, source) {
		return nil, RunnerInfo{}, nil, fmt.Errorf("unsupported source %q", source)
	}

	runner := detectRunner()
	if !runner.Available {
		return nil, RunnerInfo{}, nil, errors.New(runner.Message)
	}

	args := make([]string, 0, len(runner.Args)+10)
	args = append(args, runner.Args...)
	if source != "all" {
		args = append(args, source)
	}
	args = append(args, "session", "--json", "--no-color")
	if req.Since != "" {
		args = append(args, "--since", req.Since)
	}
	if req.Until != "" {
		args = append(args, "--until", req.Until)
	}
	if req.Offline {
		args = append(args, "--offline")
	}
	if req.NoCost {
		args = append(args, "--no-cost")
	}

	ctx, cancel := context.WithTimeout(a.ctx, 120*time.Second)
	defer cancel()

	cmd := exec.CommandContext(ctx, runner.Path, args...)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		message := strings.TrimSpace(stderr.String())
		if message == "" {
			message = strings.TrimSpace(stdout.String())
		}
		if message == "" {
			message = err.Error()
		}
		return nil, RunnerInfo{}, nil, fmt.Errorf("ccusage index scan failed: %s", message)
	}

	jsonBytes, err := extractJSONObject(stdout.Bytes())
	if err != nil {
		return nil, RunnerInfo{}, nil, err
	}

	rows, _, err := normalizeReport("session", jsonBytes)
	if err != nil {
		return nil, RunnerInfo{}, nil, err
	}
	if source != "all" {
		for index := range rows {
			if rows[index].Agent == "" {
				rows[index].Agent = source
			}
		}
	}

	return rows, runner, append([]string{runner.Path}, args...), nil
}

func openIndexDB() (*sql.DB, string, error) {
	cacheDir, err := os.UserCacheDir()
	if err != nil {
		return nil, "", err
	}

	appDir := filepath.Join(cacheDir, "ccusage-ui")
	if err := os.MkdirAll(appDir, 0o755); err != nil {
		return nil, "", err
	}

	dbPath := filepath.Join(appDir, "usage-index.sqlite")
	db, err := sql.Open("sqlite", dbPath+"?_pragma=busy_timeout(5000)")
	if err != nil {
		return nil, "", err
	}
	// Keep more than one connection available so helper functions cannot deadlock
	// if a query is opened while another result set is still being scanned.
	db.SetMaxOpenConns(4)
	db.SetMaxIdleConns(4)

	return db, dbPath, nil
}

func migrateIndexDB(db *sql.DB) error {
	statements := []string{
		`PRAGMA journal_mode = WAL`,
		`CREATE TABLE IF NOT EXISTS index_meta (
			key TEXT PRIMARY KEY,
			value TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS project_sessions (
			session_key TEXT PRIMARY KEY,
			session_id TEXT NOT NULL,
			agent TEXT NOT NULL,
			project_path TEXT NOT NULL,
			project_name TEXT NOT NULL,
			last_activity TEXT NOT NULL,
			input_tokens INTEGER NOT NULL,
			output_tokens INTEGER NOT NULL,
			cache_creation_tokens INTEGER NOT NULL,
			cache_read_tokens INTEGER NOT NULL,
			total_tokens INTEGER NOT NULL,
			total_cost REAL NOT NULL,
			models_json TEXT NOT NULL,
			raw_json TEXT NOT NULL,
			indexed_at TEXT NOT NULL
		)`,
		`CREATE TABLE IF NOT EXISTS session_models (
			session_key TEXT NOT NULL,
			model_name TEXT NOT NULL,
			input_tokens INTEGER NOT NULL,
			output_tokens INTEGER NOT NULL,
			cache_creation_tokens INTEGER NOT NULL,
			cache_read_tokens INTEGER NOT NULL,
			cost REAL NOT NULL,
			PRIMARY KEY (session_key, model_name)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_project_sessions_project ON project_sessions(project_path)`,
		`CREATE INDEX IF NOT EXISTS idx_project_sessions_project_activity ON project_sessions(project_path, last_activity DESC)`,
		`CREATE INDEX IF NOT EXISTS idx_project_sessions_agent ON project_sessions(agent)`,
		`CREATE INDEX IF NOT EXISTS idx_project_sessions_last_activity ON project_sessions(last_activity)`,
		`CREATE INDEX IF NOT EXISTS idx_session_models_model ON session_models(model_name)`,
		`CREATE INDEX IF NOT EXISTS idx_session_models_session ON session_models(session_key)`,
	}

	for _, statement := range statements {
		if _, err := db.Exec(statement); err != nil {
			return err
		}
	}
	if err := ensureColumn(db, "project_sessions", "logical_project_path", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(db, "project_sessions", "logical_project_name", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(db, "project_sessions", "grouping_rule", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if _, err := db.Exec(`UPDATE project_sessions SET
		logical_project_path = CASE WHEN logical_project_path = '' THEN project_path ELSE logical_project_path END,
		logical_project_name = CASE WHEN logical_project_name = '' THEN project_name ELSE logical_project_name END,
		grouping_rule = CASE WHEN grouping_rule = '' THEN 'physical path' ELSE grouping_rule END`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_project_sessions_logical_project ON project_sessions(logical_project_path)`); err != nil {
		return err
	}
	if _, err := db.Exec(`CREATE INDEX IF NOT EXISTS idx_project_sessions_logical_project_activity ON project_sessions(logical_project_path, last_activity DESC)`); err != nil {
		return err
	}
	if err := ensureColumn(db, "project_sessions", "last_user_message_preview", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(db, "project_sessions", "last_user_message_at", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(db, "project_sessions", "message_source_path", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(db, "project_sessions", "active_duration_seconds", "INTEGER NOT NULL DEFAULT 0"); err != nil {
		return err
	}
	if err := ensureColumn(db, "project_sessions", "originator", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(db, "project_sessions", "client_source", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(db, "project_sessions", "model", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(db, "project_sessions", "provider", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(db, "project_sessions", "reasoning_level", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	if err := ensureColumn(db, "project_sessions", "activity_json", "TEXT NOT NULL DEFAULT ''"); err != nil {
		return err
	}
	return nil
}

func ensureColumn(db *sql.DB, tableName string, columnName string, definition string) error {
	rows, err := db.Query(`PRAGMA table_info(` + tableName + `)`)
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var cid int
		var name string
		var columnType string
		var notNull int
		var defaultValue any
		var pk int
		if err := rows.Scan(&cid, &name, &columnType, &notNull, &defaultValue, &pk); err != nil {
			return err
		}
		if name == columnName {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	_, err = db.Exec(`ALTER TABLE ` + tableName + ` ADD COLUMN ` + columnName + ` ` + definition)
	return err
}

func replaceIndexedSessions(db *sql.DB, rows []ReportRow, indexedAt string) error {
	config, err := loadAppConfig()
	if err != nil {
		return err
	}
	groupingRules := effectiveProjectGroupingRules(config)

	tx, err := db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(`DELETE FROM session_models`); err != nil {
		return err
	}
	if _, err := tx.Exec(`DELETE FROM project_sessions`); err != nil {
		return err
	}

	insertSession, err := tx.Prepare(`INSERT INTO project_sessions (
		session_key, session_id, agent, project_path, project_name, logical_project_path, logical_project_name, grouping_rule, last_activity,
		input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens,
		total_tokens, total_cost, models_json, raw_json, indexed_at, originator, client_source, model, provider, reasoning_level, activity_json
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insertSession.Close()

	insertModel, err := tx.Prepare(`INSERT INTO session_models (
		session_key, model_name, input_tokens, output_tokens, cache_creation_tokens, cache_read_tokens, cost
	) VALUES (?, ?, ?, ?, ?, ?, ?)`)
	if err != nil {
		return err
	}
	defer insertModel.Close()

	// ccusage omits lastActivity (and sometimes projectPath) for some agents, so
	// lastActivity would otherwise fall back to the session UUID and projects
	// would group under "(unknown)". Scan the relevant transcripts once, lazily,
	// to backfill real values per agent.
	var geminiMeta map[string]geminiSessionMeta
	var geminiProjectPaths map[string]string
	geminiLoaded := false
	var droidActivity map[string]string
	droidLoaded := false

	for _, row := range rows {
		sessionID := row.Period
		agent := row.Agent
		if agent == "" {
			agent = "all"
		}
		sessionKey := agent + ":" + sessionID
		if agent == "gemini" && !geminiLoaded {
			geminiMeta = readGeminiSessionMeta()
			targetHashes := map[string]bool{}
			for _, meta := range geminiMeta {
				if meta.projectHash != "" {
					targetHashes[meta.projectHash] = true
				}
			}
			geminiProjectPaths = resolveGeminiProjectPaths(targetHashes)
			geminiLoaded = true
		}
		if agent == "droid" && !droidLoaded {
			droidActivity = readDroidSessionMeta()
			droidLoaded = true
		}
		projectPath := metadataString(row.Metadata, "projectPath")
		transcriptMetadata := TranscriptMetadata{}
		if projectPath == "" || agent == "pi" || agent == "codex" || agent == "claude" || agent == "opencode" {
			transcriptMetadata = inferTranscriptMetadata(row)
		}
		if transcriptMetadata.CWD != "" {
			projectPath = transcriptMetadata.CWD
		}
		if agent == "gemini" {
			if meta, ok := geminiMeta[sessionID]; ok && meta.projectHash != "" {
				if path := geminiProjectPaths[meta.projectHash]; path != "" {
					projectPath = path
				}
			}
		}
		if projectPath == "" {
			projectPath = "(unknown)"
		}
		originator := firstNonEmptyString(metadataString(row.Metadata, "originator"), transcriptMetadata.Originator)
		clientSource := firstNonEmptyString(metadataString(row.Metadata, "source"), transcriptMetadata.Source)
		model := firstNonEmptyString(metadataString(row.Metadata, "model"), transcriptMetadata.Model)
		provider := firstNonEmptyString(metadataString(row.Metadata, "provider"), transcriptMetadata.Provider)
		reasoningLevel := firstNonEmptyString(metadataString(row.Metadata, "reasoningLevel"), transcriptMetadata.ReasoningLevel)
		physicalProjectName := projectName(projectPath)
		grouped := groupProjectPath(projectPath, groupingRules)
		logicalProjectPath := grouped.LogicalPath
		logicalProjectName := grouped.DisplayPath
		if logicalProjectName == "" {
			logicalProjectName = projectName(logicalProjectPath)
		}
		lastActivity := metadataString(row.Metadata, "lastActivity")
		if lastActivity == "" {
			lastActivity = row.Period
		}
		if agent == "gemini" {
			if meta, ok := geminiMeta[sessionID]; ok && meta.lastActivity != "" {
				lastActivity = meta.lastActivity
			}
		}
		if agent == "droid" {
			if ts := droidActivity[sessionID]; ts != "" {
				lastActivity = ts
			}
		}

		modelsJSON, _ := json.Marshal(row.ModelsUsed)
		rawJSON, _ := json.Marshal(row.Raw)
		activity := readSessionActivity(agent, sessionID, row.TotalCost)
		activityJSON, _ := json.Marshal(activity)

		if _, err := insertSession.Exec(
			sessionKey,
			sessionID,
			agent,
			projectPath,
			physicalProjectName,
			logicalProjectPath,
			logicalProjectName,
			grouped.RuleName,
			lastActivity,
			row.InputTokens,
			row.OutputTokens,
			row.CacheCreationTokens,
			row.CacheReadTokens,
			row.TotalTokens,
			row.TotalCost,
			string(modelsJSON),
			string(rawJSON),
			indexedAt,
			originator,
			clientSource,
			model,
			provider,
			reasoningLevel,
			string(activityJSON),
		); err != nil {
			return err
		}

		for _, model := range row.ModelBreakdowns {
			if _, err := insertModel.Exec(
				sessionKey,
				model.ModelName,
				model.InputTokens,
				model.OutputTokens,
				model.CacheCreationTokens,
				model.CacheReadTokens,
				model.Cost,
			); err != nil {
				return err
			}
		}
	}

	if _, err := tx.Exec(`INSERT INTO index_meta(key, value) VALUES ('last_indexed', ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value`, indexedAt); err != nil {
		return err
	}

	return tx.Commit()
}

func readProjectIndex(db *sql.DB, dbPath string, filter indexFilter) (ProjectIndexResponse, error) {
	lastIndexed := ""
	_ = db.QueryRow(`SELECT value FROM index_meta WHERE key = 'last_indexed'`).Scan(&lastIndexed)

	conds, args := filter.conditions("project_sessions")
	whereClause := ""
	if conds != "" {
		whereClause = " WHERE " + conds
	}
	rows, err := db.Query(`SELECT
		logical_project_path,
		logical_project_name,
		COUNT(*) AS session_count,
		MAX(last_activity) AS last_activity,
		SUM(input_tokens) AS input_tokens,
		SUM(output_tokens) AS output_tokens,
		SUM(cache_creation_tokens) AS cache_creation_tokens,
		SUM(cache_read_tokens) AS cache_read_tokens,
		SUM(total_tokens) AS total_tokens,
		SUM(total_cost) AS total_cost,
		MIN(grouping_rule) AS grouping_rule
	FROM project_sessions`+whereClause+`
	GROUP BY logical_project_path, logical_project_name
	ORDER BY last_activity DESC, total_cost DESC, total_tokens DESC`, args...)
	if err != nil {
		return ProjectIndexResponse{}, err
	}
	defer rows.Close()

	projects := []ProjectSummary{}
	for rows.Next() {
		project := ProjectSummary{}
		if err := rows.Scan(
			&project.ProjectPath,
			&project.ProjectName,
			&project.SessionCount,
			&project.LastActivity,
			&project.InputTokens,
			&project.OutputTokens,
			&project.CacheCreationTokens,
			&project.CacheReadTokens,
			&project.TotalTokens,
			&project.TotalCost,
			&project.GroupingRule,
		); err != nil {
			return ProjectIndexResponse{}, err
		}
		projects = append(projects, project)
	}
	if err := rows.Err(); err != nil {
		return ProjectIndexResponse{}, err
	}

	agentsByProject, err := readAllProjectAgents(db, filter)
	if err != nil {
		return ProjectIndexResponse{}, err
	}
	modelsByProject, err := readAllProjectModels(db, filter)
	if err != nil {
		return ProjectIndexResponse{}, err
	}
	sessionsByProject, err := readAllProjectSessions(db, filter)
	if err != nil {
		return ProjectIndexResponse{}, err
	}
	physicalPathsByProject, err := readAllProjectPhysicalPaths(db, filter)
	if err != nil {
		return ProjectIndexResponse{}, err
	}
	for index := range projects {
		projectPath := projects[index].ProjectPath
		projects[index].Agents = agentsByProject[projectPath]
		projects[index].ModelBreakdowns = modelsByProject[projectPath]
		projects[index].RecentSessions = sessionsByProject[projectPath]
		activitySummaries := make([]ActivitySummary, 0, len(projects[index].RecentSessions))
		for _, session := range projects[index].RecentSessions {
			activitySummaries = append(activitySummaries, session.Activity)
		}
		projects[index].Activity = aggregateActivitySummaries(activitySummaries, projects[index].TotalCost)
		projects[index].PhysicalPaths = physicalPathsByProject[projectPath]
		projects[index].PathExists = projectPathExists(projectPath)
	}

	agentGroups, err := readAgentGroups(db, filter)
	if err != nil {
		return ProjectIndexResponse{}, err
	}
	modelGroups, err := readModelGroups(db, filter)
	if err != nil {
		return ProjectIndexResponse{}, err
	}

	return ProjectIndexResponse{
		Projects:    projects,
		AgentGroups: agentGroups,
		ModelGroups: modelGroups,
		Database:    dbPath,
		LastIndexed: lastIndexed,
		Generated:   time.Now().Format(time.RFC3339),
	}, nil
}

func readAllProjectAgents(db *sql.DB, filter indexFilter) (map[string][]string, error) {
	conds, args := filter.conditions("project_sessions")
	whereClause := ""
	if conds != "" {
		whereClause = " WHERE " + conds
	}
	rows, err := db.Query(`SELECT logical_project_path, agent FROM project_sessions`+whereClause+` GROUP BY logical_project_path, agent ORDER BY logical_project_path, agent`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	agentsByProject := map[string][]string{}
	for rows.Next() {
		projectPath := ""
		agent := ""
		if err := rows.Scan(&projectPath, &agent); err != nil {
			return nil, err
		}
		agentsByProject[projectPath] = append(agentsByProject[projectPath], agent)
	}
	return agentsByProject, rows.Err()
}

func readAllProjectModels(db *sql.DB, filter indexFilter) (map[string][]ModelBreakdown, error) {
	conds, args := filter.conditions("ps")
	andClause := ""
	if conds != "" {
		andClause = " AND " + conds
	}
	rows, err := db.Query(`SELECT
		ps.logical_project_path,
		sm.model_name,
		SUM(sm.input_tokens),
		SUM(sm.output_tokens),
		SUM(sm.cache_creation_tokens),
		SUM(sm.cache_read_tokens),
		SUM(sm.cost)
	FROM session_models sm
	INNER JOIN project_sessions ps ON ps.session_key = sm.session_key`+andClause+`
	GROUP BY ps.logical_project_path, sm.model_name
	ORDER BY ps.logical_project_path, SUM(sm.cost) DESC, SUM(sm.input_tokens + sm.output_tokens + sm.cache_read_tokens) DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	modelsByProject := map[string][]ModelBreakdown{}
	for rows.Next() {
		projectPath := ""
		model := ModelBreakdown{}
		if err := rows.Scan(
			&projectPath,
			&model.ModelName,
			&model.InputTokens,
			&model.OutputTokens,
			&model.CacheCreationTokens,
			&model.CacheReadTokens,
			&model.Cost,
		); err != nil {
			return nil, err
		}
		modelsByProject[projectPath] = append(modelsByProject[projectPath], model)
	}
	return modelsByProject, rows.Err()
}

func readAllProjectSessions(db *sql.DB, filter indexFilter) (map[string][]IndexedSession, error) {
	modelsBySession, err := readAllSessionModels(db)
	if err != nil {
		return nil, err
	}

	conds, args := filter.conditions("project_sessions")
	whereClause := ""
	if conds != "" {
		whereClause = " WHERE " + conds
	}
	rows, err := db.Query(`SELECT
		session_key,
		session_id,
		agent,
		project_path,
		project_name,
		logical_project_path,
		last_activity,
		input_tokens,
		output_tokens,
		cache_creation_tokens,
		cache_read_tokens,
		total_tokens,
		total_cost,
		last_user_message_preview,
		last_user_message_at,
		message_source_path,
		active_duration_seconds,
		originator,
		client_source,
		model,
		provider,
		reasoning_level,
		activity_json
	FROM project_sessions`+whereClause+`
	ORDER BY logical_project_path, last_activity DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessionsByProject := map[string][]IndexedSession{}
	for rows.Next() {
		logicalProjectPath := ""
		sessionKey := ""
		activityJSON := ""
		session := IndexedSession{}
		if err := rows.Scan(
			&sessionKey,
			&session.SessionID,
			&session.Agent,
			&session.ProjectPath,
			&session.ProjectName,
			&logicalProjectPath,
			&session.LastActivity,
			&session.InputTokens,
			&session.OutputTokens,
			&session.CacheCreationTokens,
			&session.CacheReadTokens,
			&session.TotalTokens,
			&session.TotalCost,
			&session.LastUserMessage,
			&session.LastUserMessageAt,
			&session.MessageSourcePath,
			&session.ActiveDurationSeconds,
			&session.Originator,
			&session.ClientSource,
			&session.Model,
			&session.Provider,
			&session.ReasoningLevel,
			&activityJSON,
		); err != nil {
			return nil, err
		}
		session.ModelBreakdowns = modelsBySession[sessionKey]
		_ = json.Unmarshal([]byte(activityJSON), &session.Activity)
		sessionsByProject[logicalProjectPath] = append(sessionsByProject[logicalProjectPath], session)
	}
	return sessionsByProject, rows.Err()
}

func readAllProjectPhysicalPaths(db *sql.DB, filter indexFilter) (map[string][]string, error) {
	conds, args := filter.conditions("project_sessions")
	whereClause := ""
	if conds != "" {
		whereClause = " WHERE " + conds
	}
	rows, err := db.Query(`SELECT logical_project_path, project_path
	FROM project_sessions`+whereClause+`
	GROUP BY logical_project_path, project_path
	ORDER BY logical_project_path, project_path`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	pathsByProject := map[string][]string{}
	for rows.Next() {
		logicalProjectPath := ""
		physicalPath := ""
		if err := rows.Scan(&logicalProjectPath, &physicalPath); err != nil {
			return nil, err
		}
		pathsByProject[logicalProjectPath] = append(pathsByProject[logicalProjectPath], decodeProjectPathForGrouping(physicalPath))
	}
	return pathsByProject, rows.Err()
}

func readAllSessionModels(db *sql.DB) (map[string][]ModelBreakdown, error) {
	rows, err := db.Query(`SELECT
		session_key,
		model_name,
		input_tokens,
		output_tokens,
		cache_creation_tokens,
		cache_read_tokens,
		cost
	FROM session_models
	ORDER BY session_key, cost DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	modelsBySession := map[string][]ModelBreakdown{}
	for rows.Next() {
		sessionKey := ""
		model := ModelBreakdown{}
		if err := rows.Scan(
			&sessionKey,
			&model.ModelName,
			&model.InputTokens,
			&model.OutputTokens,
			&model.CacheCreationTokens,
			&model.CacheReadTokens,
			&model.Cost,
		); err != nil {
			return nil, err
		}
		modelsBySession[sessionKey] = append(modelsBySession[sessionKey], model)
	}
	return modelsBySession, rows.Err()
}

func readProjectAgents(db *sql.DB, projectPath string) ([]string, error) {
	rows, err := db.Query(`SELECT DISTINCT agent FROM project_sessions WHERE project_path = ? ORDER BY agent`, projectPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	agents := []string{}
	for rows.Next() {
		agent := ""
		if err := rows.Scan(&agent); err != nil {
			return nil, err
		}
		agents = append(agents, agent)
	}
	return agents, rows.Err()
}

func readProjectSessions(db *sql.DB, projectPath string) ([]IndexedSession, error) {
	rows, err := db.Query(`SELECT
		session_key,
		session_id,
		agent,
		project_path,
		project_name,
		last_activity,
		input_tokens,
		output_tokens,
		cache_creation_tokens,
		cache_read_tokens,
		total_tokens,
		total_cost,
		last_user_message_preview,
		last_user_message_at,
		message_source_path,
		active_duration_seconds,
		originator,
		client_source,
		model,
		provider,
		reasoning_level,
		activity_json
	FROM project_sessions
	WHERE project_path = ?
	ORDER BY last_activity DESC
	LIMIT 12`, projectPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	sessions := []IndexedSession{}
	for rows.Next() {
		sessionKey := ""
		activityJSON := ""
		session := IndexedSession{}
		if err := rows.Scan(
			&sessionKey,
			&session.SessionID,
			&session.Agent,
			&session.ProjectPath,
			&session.ProjectName,
			&session.LastActivity,
			&session.InputTokens,
			&session.OutputTokens,
			&session.CacheCreationTokens,
			&session.CacheReadTokens,
			&session.TotalTokens,
			&session.TotalCost,
			&session.LastUserMessage,
			&session.LastUserMessageAt,
			&session.MessageSourcePath,
			&session.ActiveDurationSeconds,
			&session.Originator,
			&session.ClientSource,
			&session.Model,
			&session.Provider,
			&session.ReasoningLevel,
			&activityJSON,
		); err != nil {
			return nil, err
		}
		_ = json.Unmarshal([]byte(activityJSON), &session.Activity)
		models, err := readSessionModels(db, sessionKey)
		if err != nil {
			return nil, err
		}
		session.ModelBreakdowns = models
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

func readSessionModels(db *sql.DB, sessionKey string) ([]ModelBreakdown, error) {
	rows, err := db.Query(`SELECT
		model_name,
		input_tokens,
		output_tokens,
		cache_creation_tokens,
		cache_read_tokens,
		cost
	FROM session_models
	WHERE session_key = ?
	ORDER BY cost DESC`, sessionKey)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	models := []ModelBreakdown{}
	for rows.Next() {
		model := ModelBreakdown{}
		if err := rows.Scan(
			&model.ModelName,
			&model.InputTokens,
			&model.OutputTokens,
			&model.CacheCreationTokens,
			&model.CacheReadTokens,
			&model.Cost,
		); err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, rows.Err()
}

func readAgentGroups(db *sql.DB, filter indexFilter) ([]IndexGroup, error) {
	conds, args := filter.conditions("project_sessions")
	whereClause := ""
	if conds != "" {
		whereClause = " WHERE " + conds
	}
	rows, err := db.Query(`SELECT
		agent,
		COUNT(DISTINCT project_path),
		COUNT(*),
		MAX(last_activity),
		SUM(input_tokens),
		SUM(output_tokens),
		SUM(cache_creation_tokens),
		SUM(cache_read_tokens),
		SUM(total_tokens),
		SUM(total_cost)
	FROM project_sessions`+whereClause+`
	GROUP BY agent
	ORDER BY SUM(total_cost) DESC, SUM(total_tokens) DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := []IndexGroup{}
	for rows.Next() {
		group := IndexGroup{GroupBy: "agent"}
		if err := rows.Scan(
			&group.Name,
			&group.ProjectCount,
			&group.SessionCount,
			&group.LastActivity,
			&group.InputTokens,
			&group.OutputTokens,
			&group.CacheCreationTokens,
			&group.CacheReadTokens,
			&group.TotalTokens,
			&group.TotalCost,
		); err != nil {
			return nil, err
		}
		group.Agents = []string{group.Name}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	modelsByAgent, err := readAllAgentModels(db, filter)
	if err != nil {
		return nil, err
	}
	for index := range groups {
		groups[index].ModelBreakdowns = modelsByAgent[groups[index].Name]
	}
	return groups, nil
}

func readAllAgentModels(db *sql.DB, filter indexFilter) (map[string][]ModelBreakdown, error) {
	conds, args := filter.conditions("ps")
	andClause := ""
	if conds != "" {
		andClause = " AND " + conds
	}
	rows, err := db.Query(`SELECT
		ps.agent,
		sm.model_name,
		SUM(sm.input_tokens),
		SUM(sm.output_tokens),
		SUM(sm.cache_creation_tokens),
		SUM(sm.cache_read_tokens),
		SUM(sm.cost)
	FROM session_models sm
	INNER JOIN project_sessions ps ON ps.session_key = sm.session_key`+andClause+`
	GROUP BY ps.agent, sm.model_name
	ORDER BY ps.agent, SUM(sm.cost) DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	modelsByAgent := map[string][]ModelBreakdown{}
	for rows.Next() {
		agent := ""
		model := ModelBreakdown{}
		if err := rows.Scan(
			&agent,
			&model.ModelName,
			&model.InputTokens,
			&model.OutputTokens,
			&model.CacheCreationTokens,
			&model.CacheReadTokens,
			&model.Cost,
		); err != nil {
			return nil, err
		}
		modelsByAgent[agent] = append(modelsByAgent[agent], model)
	}
	return modelsByAgent, rows.Err()
}

func readModelGroups(db *sql.DB, filter indexFilter) ([]IndexGroup, error) {
	conds, args := filter.conditions("ps")
	andClause := ""
	if conds != "" {
		andClause = " AND " + conds
	}
	rows, err := db.Query(`SELECT
		sm.model_name,
		COUNT(DISTINCT ps.project_path),
		COUNT(DISTINCT ps.session_key),
		MAX(ps.last_activity),
		SUM(sm.input_tokens),
		SUM(sm.output_tokens),
		SUM(sm.cache_creation_tokens),
		SUM(sm.cache_read_tokens),
		SUM(sm.input_tokens + sm.output_tokens + sm.cache_creation_tokens + sm.cache_read_tokens),
		SUM(sm.cost)
	FROM session_models sm
	INNER JOIN project_sessions ps ON ps.session_key = sm.session_key`+andClause+`
	GROUP BY sm.model_name
	ORDER BY SUM(sm.cost) DESC, SUM(sm.input_tokens + sm.output_tokens + sm.cache_read_tokens) DESC`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	groups := []IndexGroup{}
	for rows.Next() {
		group := IndexGroup{GroupBy: "model"}
		if err := rows.Scan(
			&group.Name,
			&group.ProjectCount,
			&group.SessionCount,
			&group.LastActivity,
			&group.InputTokens,
			&group.OutputTokens,
			&group.CacheCreationTokens,
			&group.CacheReadTokens,
			&group.TotalTokens,
			&group.TotalCost,
		); err != nil {
			return nil, err
		}
		group.ModelBreakdowns = []ModelBreakdown{
			{
				ModelName:           group.Name,
				InputTokens:         group.InputTokens,
				OutputTokens:        group.OutputTokens,
				CacheCreationTokens: group.CacheCreationTokens,
				CacheReadTokens:     group.CacheReadTokens,
				Cost:                group.TotalCost,
			},
		}
		groups = append(groups, group)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	agentsByModel, err := readAllModelAgents(db, filter)
	if err != nil {
		return nil, err
	}
	for index := range groups {
		groups[index].Agents = agentsByModel[groups[index].Name]
	}
	return groups, nil
}

func readAllModelAgents(db *sql.DB, filter indexFilter) (map[string][]string, error) {
	conds, args := filter.conditions("ps")
	andClause := ""
	if conds != "" {
		andClause = " AND " + conds
	}
	rows, err := db.Query(`SELECT sm.model_name, ps.agent
	FROM project_sessions ps
	INNER JOIN session_models sm ON sm.session_key = ps.session_key`+andClause+`
	GROUP BY sm.model_name, ps.agent
	ORDER BY sm.model_name, ps.agent`, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	agentsByModel := map[string][]string{}
	for rows.Next() {
		modelName := ""
		agent := ""
		if err := rows.Scan(&modelName, &agent); err != nil {
			return nil, err
		}
		agentsByModel[modelName] = append(agentsByModel[modelName], agent)
	}
	return agentsByModel, rows.Err()
}

func readProjectModels(db *sql.DB, projectPath string) ([]ModelBreakdown, error) {
	rows, err := db.Query(`SELECT
		sm.model_name,
		SUM(sm.input_tokens),
		SUM(sm.output_tokens),
		SUM(sm.cache_creation_tokens),
		SUM(sm.cache_read_tokens),
		SUM(sm.cost)
	FROM session_models sm
	INNER JOIN project_sessions ps ON ps.session_key = sm.session_key
	WHERE ps.project_path = ?
	GROUP BY sm.model_name
	ORDER BY SUM(sm.cost) DESC, SUM(sm.input_tokens + sm.output_tokens + sm.cache_read_tokens) DESC`, projectPath)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return scanModelBreakdowns(rows)
}

func scanModelBreakdowns(rows *sql.Rows) ([]ModelBreakdown, error) {
	models := []ModelBreakdown{}
	for rows.Next() {
		model := ModelBreakdown{}
		if err := rows.Scan(
			&model.ModelName,
			&model.InputTokens,
			&model.OutputTokens,
			&model.CacheCreationTokens,
			&model.CacheReadTokens,
			&model.Cost,
		); err != nil {
			return nil, err
		}
		models = append(models, model)
	}
	return models, rows.Err()
}

func inferTranscriptMetadata(row ReportRow) TranscriptMetadata {
	agent := row.Agent
	if agent == "" {
		if strings.Contains(row.Period, "rollout-") {
			agent = "codex"
		}
	}
	if agent == "" || row.Period == "" {
		return TranscriptMetadata{}
	}
	sourcePath, err := locateTranscript(agent, row.Period, "")
	if err != nil {
		return TranscriptMetadata{}
	}
	return extractTranscriptMetadata(sourcePath)
}

func inferProjectPathFromTranscript(row ReportRow) string {
	return inferTranscriptMetadata(row).CWD
}

func (a *App) OpenProjectInFinder(projectPath string) error {
	path, err := resolveProjectPath(projectPath)
	if err != nil {
		return err
	}

	if _, err := os.Stat(path); err == nil {
		return exec.Command("open", "-R", path).Run()
	}

	parent := nearestExistingParent(path)
	if parent == "" {
		return fmt.Errorf("could not find project path %q", projectPath)
	}
	return exec.Command("open", parent).Run()
}

func (a *App) OpenPathInFinder(path string) error {
	if path == "" {
		return fmt.Errorf("path is empty")
	}
	if _, err := os.Stat(path); err == nil {
		return exec.Command("open", "-R", path).Run()
	}
	parent := nearestExistingParent(path)
	if parent == "" {
		return fmt.Errorf("could not find path %q", path)
	}
	return exec.Command("open", parent).Run()
}

func projectPathExists(projectPath string) bool {
	path, err := resolveProjectPath(projectPath)
	if err != nil {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && info.IsDir()
}

func resolveProjectPath(projectPath string) (string, error) {
	if projectPath == "" || projectPath == "(unknown)" {
		return "", fmt.Errorf("project path is unknown")
	}
	if filepath.IsAbs(projectPath) {
		return projectPath, nil
	}

	cleaned := strings.Trim(projectPath, "-")
	if cleaned == "" {
		return "", fmt.Errorf("project path is unknown")
	}
	if resolved := resolveDashedPathFromFilesystem(cleaned); resolved != "" {
		return resolved, nil
	}
	return string(filepath.Separator) + strings.ReplaceAll(cleaned, "-", string(filepath.Separator)), nil
}

func resolveDashedPathFromFilesystem(cleaned string) string {
	parts := strings.Split(cleaned, "-")
	current := string(filepath.Separator)
	for index := 0; index < len(parts); {
		matched := ""
		matchedEnd := index
		for end := len(parts); end > index; end-- {
			candidateName := strings.Join(parts[index:end], "-")
			candidatePath := filepath.Join(current, candidateName)
			if info, err := os.Stat(candidatePath); err == nil && info.IsDir() {
				matched = candidatePath
				matchedEnd = end
				break
			}
		}
		if matched == "" {
			return ""
		}
		current = matched
		index = matchedEnd
	}
	return current
}

func nearestExistingParent(path string) string {
	for {
		path = filepath.Dir(path)
		if path == "." || path == string(filepath.Separator) {
			return ""
		}
		if info, err := os.Stat(path); err == nil && info.IsDir() {
			return path
		}
	}
}

func metadataString(metadata map[string]any, key string) string {
	if metadata == nil {
		return ""
	}
	value, ok := metadata[key]
	if !ok {
		return ""
	}
	text, ok := value.(string)
	if !ok {
		return ""
	}
	return text
}

func projectName(projectPath string) string {
	if projectPath == "(unknown)" {
		return projectPath
	}

	if filepath.IsAbs(projectPath) {
		name := filepath.Base(filepath.Clean(projectPath))
		if name == "." || name == string(filepath.Separator) {
			return projectPath
		}
		return name
	}

	cleaned := strings.Trim(projectPath, "-")
	cleaned = strings.ReplaceAll(cleaned, "-", string(filepath.Separator))
	cleaned = filepath.Clean(cleaned)
	name := filepath.Base(cleaned)
	if name == "." || name == string(filepath.Separator) {
		return projectPath
	}
	return name
}
