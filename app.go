package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
	"slices"
	"strings"
	"time"
)

type App struct {
	ctx context.Context
}

type ReportRequest struct {
	Report  string `json:"report"`
	Source  string `json:"source"`
	Since   string `json:"since"`
	Until   string `json:"until"`
	Offline bool   `json:"offline"`
	NoCost  bool   `json:"noCost"`
}

type RunnerInfo struct {
	Name      string   `json:"name"`
	Path      string   `json:"path"`
	Args      []string `json:"args"`
	Available bool     `json:"available"`
	Message   string   `json:"message"`
}

type ReportResponse struct {
	Report    string         `json:"report"`
	Source    string         `json:"source"`
	Runner    RunnerInfo     `json:"runner"`
	Command   []string       `json:"command"`
	Rows      []ReportRow    `json:"rows"`
	Totals    map[string]any `json:"totals"`
	Generated string         `json:"generated"`
}

type ReportRow struct {
	Period              string           `json:"period"`
	Agent               string           `json:"agent"`
	InputTokens         int64            `json:"inputTokens"`
	OutputTokens        int64            `json:"outputTokens"`
	CacheCreationTokens int64            `json:"cacheCreationTokens"`
	CacheReadTokens     int64            `json:"cacheReadTokens"`
	TotalTokens         int64            `json:"totalTokens"`
	TotalCost           float64          `json:"totalCost"`
	ModelsUsed          []string         `json:"modelsUsed"`
	ModelBreakdowns     []ModelBreakdown `json:"modelBreakdowns"`
	Metadata            map[string]any   `json:"metadata"`
	Raw                 map[string]any   `json:"raw"`
}

type ModelBreakdown struct {
	ModelName           string  `json:"modelName"`
	InputTokens         int64   `json:"inputTokens"`
	OutputTokens        int64   `json:"outputTokens"`
	CacheCreationTokens int64   `json:"cacheCreationTokens"`
	CacheReadTokens     int64   `json:"cacheReadTokens"`
	Cost                float64 `json:"cost"`
}

type rawReportRow struct {
	Period              string           `json:"period"`
	Agent               string           `json:"agent"`
	InputTokens         int64            `json:"inputTokens"`
	OutputTokens        int64            `json:"outputTokens"`
	CacheCreationTokens int64            `json:"cacheCreationTokens"`
	CacheReadTokens     int64            `json:"cacheReadTokens"`
	TotalTokens         int64            `json:"totalTokens"`
	TotalCost           float64          `json:"totalCost"`
	ModelsUsed          []string         `json:"modelsUsed"`
	ModelBreakdowns     []ModelBreakdown `json:"modelBreakdowns"`
	Metadata            map[string]any   `json:"metadata"`
}

var reportNames = []string{"daily", "weekly", "monthly", "session", "blocks"}
var sourceNames = []string{"all", "claude", "codex", "opencode", "amp", "droid", "codebuff", "hermes", "pi", "goose", "kilo", "copilot", "gemini", "kimi", "qwen", "openclaw"}

func NewApp() *App {
	return &App{}
}

func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

func (a *App) GetRunner() RunnerInfo {
	return detectRunner()
}

func (a *App) GetReport(req ReportRequest) (ReportResponse, error) {
	report := strings.TrimSpace(req.Report)
	if report == "" {
		report = "daily"
	}
	if !slices.Contains(reportNames, report) {
		return ReportResponse{}, fmt.Errorf("unsupported report %q", report)
	}

	source := strings.TrimSpace(req.Source)
	if source == "" {
		source = "all"
	}
	if !slices.Contains(sourceNames, source) {
		return ReportResponse{}, fmt.Errorf("unsupported source %q", source)
	}

	runner := detectRunner()
	if !runner.Available {
		return ReportResponse{}, errors.New(runner.Message)
	}

	args := make([]string, 0, len(runner.Args)+10)
	args = append(args, runner.Args...)
	if source != "all" {
		args = append(args, source)
	}
	args = append(args, report, "--json", "--no-color")

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

	ctx, cancel := context.WithTimeout(a.ctx, 90*time.Second)
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
		return ReportResponse{}, fmt.Errorf("ccusage failed: %s", message)
	}

	jsonBytes, err := extractJSONObject(stdout.Bytes())
	if err != nil {
		return ReportResponse{}, err
	}

	rows, totals, err := normalizeReport(report, jsonBytes)
	if err != nil {
		return ReportResponse{}, err
	}

	command := append([]string{runner.Path}, args...)
	return ReportResponse{
		Report:    report,
		Source:    source,
		Runner:    runner,
		Command:   command,
		Rows:      rows,
		Totals:    totals,
		Generated: time.Now().Format(time.RFC3339),
	}, nil
}

func detectRunner() RunnerInfo {
	if path, err := exec.LookPath("ccusage"); err == nil {
		return RunnerInfo{
			Name:      "ccusage",
			Path:      path,
			Available: true,
			Message:   "Using ccusage from PATH.",
		}
	}

	if path, err := exec.LookPath("bunx"); err == nil {
		return RunnerInfo{
			Name:      "bunx",
			Path:      path,
			Args:      []string{"ccusage"},
			Available: true,
			Message:   "Using bunx ccusage.",
		}
	}

	if path, err := exec.LookPath("nix"); err == nil {
		return RunnerInfo{
			Name:      "nix",
			Path:      path,
			Args:      []string{"run", "github:ccusage/ccusage", "--"},
			Available: true,
			Message:   "Using nix run github:ccusage/ccusage.",
		}
	}

	if path, err := exec.LookPath("npx"); err == nil {
		return RunnerInfo{
			Name:      "npx",
			Path:      path,
			Args:      []string{"ccusage@latest"},
			Available: true,
			Message:   "Using npx ccusage@latest.",
		}
	}

	if path, err := exec.LookPath("pnpm"); err == nil {
		return RunnerInfo{
			Name:      "pnpm",
			Path:      path,
			Args:      []string{"dlx", "ccusage"},
			Available: true,
			Message:   "Using pnpm dlx ccusage.",
		}
	}

	return RunnerInfo{
		Available: false,
		Message:   "Could not find ccusage, bunx, nix, npx, or pnpm on PATH.",
	}
}

func extractJSONObject(output []byte) ([]byte, error) {
	start := bytes.IndexByte(output, '{')
	end := bytes.LastIndexByte(output, '}')
	if start < 0 || end < start {
		return nil, fmt.Errorf("ccusage did not return JSON output")
	}
	return output[start : end+1], nil
}

func normalizeReport(report string, data []byte) ([]ReportRow, map[string]any, error) {
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(data, &payload); err != nil {
		return nil, nil, fmt.Errorf("could not parse ccusage JSON: %w", err)
	}

	rawRows, ok := payload[report]
	if !ok {
		for key, value := range payload {
			if key != "totals" {
				rawRows = value
				ok = true
				break
			}
		}
	}
	if !ok {
		return nil, nil, fmt.Errorf("ccusage JSON did not contain report rows")
	}

	var decodedRows []rawReportRow
	if err := json.Unmarshal(rawRows, &decodedRows); err != nil {
		return nil, nil, fmt.Errorf("could not parse %s rows: %w", report, err)
	}

	var rawMaps []map[string]any
	_ = json.Unmarshal(rawRows, &rawMaps)

	rows := make([]ReportRow, 0, len(decodedRows))
	for index, row := range decodedRows {
		raw := map[string]any{}
		if index < len(rawMaps) {
			raw = rawMaps[index]
		}
		rows = append(rows, ReportRow{
			Period:              row.Period,
			Agent:               row.Agent,
			InputTokens:         row.InputTokens,
			OutputTokens:        row.OutputTokens,
			CacheCreationTokens: row.CacheCreationTokens,
			CacheReadTokens:     row.CacheReadTokens,
			TotalTokens:         row.TotalTokens,
			TotalCost:           row.TotalCost,
			ModelsUsed:          row.ModelsUsed,
			ModelBreakdowns:     row.ModelBreakdowns,
			Metadata:            row.Metadata,
			Raw:                 raw,
		})
	}

	totals := map[string]any{}
	if rawTotals, ok := payload["totals"]; ok {
		_ = json.Unmarshal(rawTotals, &totals)
	}

	return rows, totals, nil
}
