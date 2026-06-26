package main

import (
	"database/sql"
	"path/filepath"
	"testing"

	_ "modernc.org/sqlite"
)

func TestProjectIndexAggregatesSessions(t *testing.T) {
	dbPath := filepath.Join(t.TempDir(), "usage-index.sqlite")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	if err := migrateIndexDB(db); err != nil {
		t.Fatal(err)
	}

	rows := []ReportRow{
		{
			Period:          "session-1",
			Agent:           "codex",
			InputTokens:     10,
			OutputTokens:    5,
			CacheReadTokens: 20,
			TotalTokens:     35,
			TotalCost:       1.25,
			ModelsUsed:      []string{"gpt-5.5"},
			Metadata: map[string]any{
				"projectPath":  "--Users-stan-workspace-ccusage-ui--",
				"lastActivity": "2026-06-25T10:00:00Z",
			},
			ModelBreakdowns: []ModelBreakdown{
				{ModelName: "gpt-5.5", InputTokens: 10, OutputTokens: 5, CacheReadTokens: 20, Cost: 1.25},
			},
		},
		{
			Period:          "session-2",
			Agent:           "pi",
			InputTokens:     15,
			OutputTokens:    10,
			CacheReadTokens: 30,
			TotalTokens:     55,
			TotalCost:       2.75,
			ModelsUsed:      []string{"gpt-5.5"},
			Metadata: map[string]any{
				"projectPath":  "--Users-stan-workspace-ccusage-ui--",
				"lastActivity": "2026-06-25T11:00:00Z",
			},
			ModelBreakdowns: []ModelBreakdown{
				{ModelName: "gpt-5.5", InputTokens: 15, OutputTokens: 10, CacheReadTokens: 30, Cost: 2.75},
			},
		},
	}

	if err := replaceIndexedSessions(db, rows, "2026-06-25T12:00:00Z"); err != nil {
		t.Fatal(err)
	}

	index, err := readProjectIndex(db, dbPath)
	if err != nil {
		t.Fatal(err)
	}

	if len(index.Projects) != 1 {
		t.Fatalf("expected one project, got %d", len(index.Projects))
	}

	project := index.Projects[0]
	if project.SessionCount != 2 {
		t.Fatalf("expected two sessions, got %d", project.SessionCount)
	}
	if project.TotalTokens != 90 {
		t.Fatalf("expected 90 tokens, got %d", project.TotalTokens)
	}
	if project.TotalCost != 4 {
		t.Fatalf("expected cost 4, got %f", project.TotalCost)
	}
	if len(project.Agents) != 2 {
		t.Fatalf("expected two agents, got %#v", project.Agents)
	}
	if len(project.ModelBreakdowns) != 1 {
		t.Fatalf("expected one model breakdown, got %d", len(project.ModelBreakdowns))
	}
	if project.ModelBreakdowns[0].InputTokens != 25 {
		t.Fatalf("expected 25 model input tokens, got %d", project.ModelBreakdowns[0].InputTokens)
	}
	if len(project.RecentSessions) != 2 {
		t.Fatalf("expected two recent sessions, got %d", len(project.RecentSessions))
	}
	if len(index.AgentGroups) != 2 {
		t.Fatalf("expected two agent groups, got %d", len(index.AgentGroups))
	}
	if len(index.ModelGroups) != 1 {
		t.Fatalf("expected one model group, got %d", len(index.ModelGroups))
	}
	if index.ModelGroups[0].ProjectCount != 1 {
		t.Fatalf("expected model group to include one project, got %d", index.ModelGroups[0].ProjectCount)
	}
}
