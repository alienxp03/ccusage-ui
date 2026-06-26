package main

import (
	"os"
	"path/filepath"
	"strings"
)

type groupedProjectPath struct {
	LogicalPath string
	DisplayPath string
	RuleName    string
}

func groupProjectPath(projectPath string, rules []ProjectGroupingRule) groupedProjectPath {
	decoded := decodeProjectPathForGrouping(projectPath)
	for _, rule := range rules {
		captures, ok := matchGroupingPattern(rule.MatchPath, decoded)
		if !ok {
			continue
		}
		logicalPath := renderGroupingPath(rule.GroupAs, captures)
		displayPath := ""
		if strings.TrimSpace(rule.DisplayAs) != "" {
			displayPath = renderGroupingDisplayPath(rule.DisplayAs, captures)
		}
		return groupedProjectPath{LogicalPath: logicalPath, DisplayPath: displayPath, RuleName: rule.Name}
	}
	return groupedProjectPath{LogicalPath: decoded, RuleName: "physical path"}
}

func decodeProjectPathForGrouping(value string) string {
	if value == "" || value == "(unknown)" {
		return value
	}
	if filepath.IsAbs(value) {
		return filepath.Clean(value)
	}
	if strings.HasPrefix(value, "--") {
		cleaned := strings.Trim(value, "-")
		if resolved := resolveDashedPathFromFilesystem(cleaned); resolved != "" {
			return filepath.Clean(resolved)
		}
		parts := strings.Split(cleaned, "-")
		return string(filepath.Separator) + filepath.Join(parts...)
	}
	return value
}

func matchGroupingPattern(pattern string, path string) (map[string][]string, bool) {
	patternSegments := groupingSegments(expandGroupingHome(pattern))
	pathSegments := groupingSegments(path)
	captures := map[string][]string{}
	if matchGroupingSegments(patternSegments, pathSegments, captures) {
		return captures, true
	}
	return nil, false
}

func matchGroupingSegments(pattern []string, path []string, captures map[string][]string) bool {
	if len(pattern) == 0 {
		return len(path) == 0
	}
	segment := pattern[0]
	name, variadic := groupingPlaceholder(segment)
	if name == "" {
		return len(path) > 0 && segment == path[0] && matchGroupingSegments(pattern[1:], path[1:], captures)
	}
	if !variadic {
		if len(path) == 0 {
			return false
		}
		captures[name] = []string{path[0]}
		return matchGroupingSegments(pattern[1:], path[1:], captures)
	}
	if len(pattern) == 1 {
		captures[name] = append([]string{}, path...)
		return true
	}
	for end := 0; end <= len(path); end++ {
		nextCaptures := cloneCaptures(captures)
		nextCaptures[name] = append([]string{}, path[:end]...)
		if matchGroupingSegments(pattern[1:], path[end:], nextCaptures) {
			for key, value := range nextCaptures {
				captures[key] = value
			}
			return true
		}
	}
	return false
}

func renderGroupingPath(template string, captures map[string][]string) string {
	segments := groupingSegments(expandGroupingHome(template))
	output := []string{}
	for _, segment := range segments {
		name, variadic := groupingPlaceholder(segment)
		if name == "" {
			output = append(output, segment)
			continue
		}
		values := captures[name]
		if !variadic && len(values) > 1 {
			output = append(output, strings.Join(values, "-"))
			continue
		}
		output = append(output, values...)
	}
	return string(filepath.Separator) + filepath.Join(output...)
}

func renderGroupingDisplayPath(template string, captures map[string][]string) string {
	segments := groupingSegments(expandGroupingHome(template))
	output := []string{}
	for _, segment := range segments {
		name, variadic := groupingPlaceholder(segment)
		if name == "" {
			output = append(output, segment)
			continue
		}
		values := captures[name]
		if !variadic && len(values) > 1 {
			output = append(output, strings.Join(values, "-"))
			continue
		}
		output = append(output, values...)
	}
	return filepath.Join(output...)
}

func groupingSegments(value string) []string {
	cleaned := filepath.Clean(value)
	cleaned = strings.TrimPrefix(cleaned, string(filepath.Separator))
	if cleaned == "." || cleaned == "" {
		return nil
	}
	return strings.Split(cleaned, string(filepath.Separator))
}

func groupingPlaceholder(segment string) (string, bool) {
	if !strings.HasPrefix(segment, "{") || !strings.HasSuffix(segment, "}") {
		return "", false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(segment, "{"), "}")
	if strings.HasSuffix(name, "...") {
		return strings.TrimSuffix(name, "..."), true
	}
	return name, false
}

func expandGroupingHome(value string) string {
	home, err := os.UserHomeDir()
	if err != nil {
		return value
	}
	return strings.ReplaceAll(value, "{home}", home)
}

func cloneCaptures(captures map[string][]string) map[string][]string {
	cloned := map[string][]string{}
	for key, value := range captures {
		cloned[key] = append([]string{}, value...)
	}
	return cloned
}
