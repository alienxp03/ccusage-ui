package app

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type AppConfig struct {
	ProjectGrouping ProjectGroupingConfig `json:"projectGrouping"`
}

type ProjectGroupingConfig struct {
	Enabled bool                  `json:"enabled"`
	Rules   []ProjectGroupingRule `json:"rules"`
}

type ProjectGroupingRule struct {
	Name      string `json:"name"`
	MatchPath string `json:"matchPath"`
	GroupAs   string `json:"groupAs"`
	DisplayAs string `json:"displayAs,omitempty"`
	Pattern   string `json:"pattern,omitempty"`
	GroupPath string `json:"groupPath,omitempty"`
}

func defaultAppConfig() AppConfig {
	return AppConfig{ProjectGrouping: ProjectGroupingConfig{Enabled: true, Rules: defaultProjectGroupingRules()}}
}

func defaultProjectGroupingRules() []ProjectGroupingRule {
	return []ProjectGroupingRule{
		{
			Name:      "Group Git worktrees by repo project",
			MatchPath: "{home}/workspace/worktrees/{owner}/{repo}/{worktree...}/projects/{subpath...}",
			GroupAs:   "{home}/workspace/{repo}/projects/{subpath...}",
			DisplayAs: "{repo}/{subpath...}",
		},
		{
			Name:      "Group repo-local .worktrees by repo project",
			MatchPath: "{home}/workspace/{repo}/.worktrees/{worktree...}/projects/{subpath...}",
			GroupAs:   "{home}/workspace/{repo}/projects/{subpath...}",
			DisplayAs: "{repo}/{subpath...}",
		},
	}
}

func (a *App) GetConfig() (AppConfig, error) {
	return loadAppConfig()
}

func (a *App) SaveConfig(config AppConfig) (AppConfig, error) {
	config = normalizeAppConfig(config)
	if err := validateAppConfig(config); err != nil {
		return AppConfig{}, err
	}
	path, err := configPath()
	if err != nil {
		return AppConfig{}, err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return AppConfig{}, err
	}
	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return AppConfig{}, err
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return AppConfig{}, err
	}
	return config, nil
}

func loadAppConfig() (AppConfig, error) {
	config := defaultAppConfig()
	path, err := configPath()
	if err != nil {
		return config, err
	}
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return config, nil
	}
	if err != nil {
		return config, err
	}
	if err := json.Unmarshal(data, &config); err != nil {
		return config, fmt.Errorf("could not parse config: %w", err)
	}
	if config.ProjectGrouping.Enabled && config.ProjectGrouping.Rules == nil {
		config.ProjectGrouping.Rules = defaultProjectGroupingRules()
	}
	config = normalizeAppConfig(config)
	if err := validateAppConfig(config); err != nil {
		return config, err
	}
	return config, nil
}

func normalizeAppConfig(config AppConfig) AppConfig {
	for index := range config.ProjectGrouping.Rules {
		rule := &config.ProjectGrouping.Rules[index]
		if rule.MatchPath == "" {
			rule.MatchPath = rule.Pattern
		}
		if rule.GroupAs == "" {
			rule.GroupAs = rule.GroupPath
		}
		rule.Pattern = ""
		rule.GroupPath = ""
	}
	return config
}

func validateAppConfig(config AppConfig) error {
	for _, rule := range config.ProjectGrouping.Rules {
		if strings.TrimSpace(rule.Name) == "" {
			return fmt.Errorf("project grouping rule name is required")
		}
		if strings.TrimSpace(rule.MatchPath) == "" || strings.TrimSpace(rule.GroupAs) == "" {
			return fmt.Errorf("project grouping rule %q needs matchPath and groupAs", rule.Name)
		}
	}
	return nil
}

func effectiveProjectGroupingRules(config AppConfig) []ProjectGroupingRule {
	if !config.ProjectGrouping.Enabled {
		return nil
	}
	return config.ProjectGrouping.Rules
}

func configPath() (string, error) {
	configDir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(configDir, "ccusage-ui", "config.json"), nil
}
