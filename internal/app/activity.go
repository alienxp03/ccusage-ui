package app

import (
	"bufio"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"
)

type ActivitySummary struct {
	Surfaces     []ActivityBreakdown `json:"surfaces"`
	Tools        []ActivityBreakdown `json:"tools"`
	Integrations []ActivityBreakdown `json:"integrations"`
	Operations   []ActivityBreakdown `json:"operations"`
	Totals       ActivityTotals      `json:"totals"`
}

type ActivityTotals struct {
	Calls           int64   `json:"calls"`
	Errors          int64   `json:"errors"`
	InputChars      int64   `json:"inputChars"`
	OutputChars     int64   `json:"outputChars"`
	EstimatedTokens int64   `json:"estimatedTokens"`
	ObservedTokens  int64   `json:"observedTokens"`
	EstimatedCost   float64 `json:"estimatedCost"`
}

type ActivityBreakdown struct {
	Surface         string  `json:"surface,omitempty"`
	ToolName        string  `json:"toolName,omitempty"`
	Provider        string  `json:"provider,omitempty"`
	Operation       string  `json:"operation,omitempty"`
	Calls           int64   `json:"calls"`
	Errors          int64   `json:"errors"`
	InputChars      int64   `json:"inputChars"`
	OutputChars     int64   `json:"outputChars"`
	EstimatedTokens int64   `json:"estimatedTokens"`
	ObservedTokens  int64   `json:"observedTokens"`
	EstimatedCost   float64 `json:"estimatedCost"`
}

type activityEvent struct {
	Surface        string
	ToolName       string
	Provider       string
	Operation      string
	InputChars     int64
	OutputChars    int64
	ObservedTokens int64
	IsError        bool
}

var originalTokenCountPattern = regexp.MustCompile(`(?m)Original token count:\s*(\d+)`)

type mcpDirectToolInfo struct {
	Server    string
	Operation string
}

func readSessionActivity(agent string, sessionID string, totalCost float64) ActivitySummary {
	if agent == "" || sessionID == "" {
		return ActivitySummary{}
	}
	path, err := locateTranscript(agent, sessionID, "")
	if err != nil || path == "" {
		return ActivitySummary{}
	}
	summary := summarizeActivityEvents(extractActivityEvents(agent, path))
	assignActivityCost(&summary, totalCost)
	return summary
}

func extractActivityEvents(agent string, path string) []activityEvent {
	piDirectMcpTools := map[string]mcpDirectToolInfo(nil)
	if agent == "pi" {
		piDirectMcpTools = loadPiDirectMcpToolMap()
	}

	file, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer file.Close()

	events := []activityEvent{}
	scanner := bufio.NewScanner(file)
	scanner.Buffer(make([]byte, 1024), 20*1024*1024)
	for scanner.Scan() {
		var line map[string]any
		if err := json.Unmarshal(scanner.Bytes(), &line); err != nil {
			continue
		}
		switch agent {
		case "pi":
			events = append(events, piActivityEvents(line, piDirectMcpTools)...)
		case "codex":
			events = append(events, codexActivityEvents(line)...)
		}
	}
	return events
}

func piActivityEvents(line map[string]any, directMcpTools map[string]mcpDirectToolInfo) []activityEvent {
	if stringValue(line["type"]) != "message" {
		return nil
	}
	message, ok := line["message"].(map[string]any)
	if !ok {
		return nil
	}
	role := stringValue(message["role"])
	if role == "toolResult" {
		toolName := stringValue(message["toolName"])
		text := messageContentText(message["content"])
		event := activityEvent{Surface: classifySurface(toolName, nil), ToolName: toolName, OutputChars: int64(len(text)), ObservedTokens: parseObservedTokens(text), IsError: boolValue(message["isError"])}
		applyPiDirectMcpInfo(&event, directMcpTools)
		return []activityEvent{event}
	}
	content, ok := message["content"].([]any)
	if !ok {
		return nil
	}
	events := []activityEvent{}
	for _, item := range content {
		part, ok := item.(map[string]any)
		if !ok || stringValue(part["type"]) != "toolCall" {
			continue
		}
		toolName := stringValue(part["name"])
		args, _ := part["arguments"].(map[string]any)
		provider, operation := providerOperation(toolName, args)
		argsJSON := compactJSONValue(part["arguments"])
		event := activityEvent{Surface: classifySurface(toolName, args), ToolName: toolName, Provider: provider, Operation: operation, InputChars: int64(len(argsJSON))}
		applyPiDirectMcpInfo(&event, directMcpTools)
		events = append(events, event)
	}
	return events
}

func codexActivityEvents(line map[string]any) []activityEvent {
	if stringValue(line["type"]) != "response_item" {
		return nil
	}
	payload, ok := line["payload"].(map[string]any)
	if !ok {
		return nil
	}
	typeName := stringValue(payload["type"])
	switch typeName {
	case "function_call":
		name := stringValue(payload["name"])
		argsText := stringValue(payload["arguments"])
		args := parseJSONObject(argsText)
		provider, operation := providerOperation(name, args)
		surface := classifySurface(name, args)
		if namespace := stringValue(payload["namespace"]); strings.HasPrefix(namespace, "mcp__") {
			surface = "mcp"
			provider = strings.TrimPrefix(namespace, "mcp__")
			operation = name
		}
		return []activityEvent{{Surface: surface, ToolName: name, Provider: provider, Operation: operation, InputChars: int64(len(argsText))}}
	case "custom_tool_call":
		name := stringValue(payload["name"])
		input := stringValue(payload["input"])
		return []activityEvent{{Surface: classifySurface(name, nil), ToolName: name, InputChars: int64(len(input))}}
	case "function_call_output", "custom_tool_call_output":
		output := stringValue(payload["output"])
		return []activityEvent{{Surface: "tool-output", ToolName: typeName, OutputChars: int64(len(output)), ObservedTokens: parseObservedTokens(output), IsError: strings.Contains(strings.ToLower(output), "exit code: 1") || strings.Contains(strings.ToLower(output), "process exited with code 1")}}
	case "web_search_call":
		input := compactJSONValue(payload["action"])
		action, _ := payload["action"].(map[string]any)
		return []activityEvent{{Surface: "web", ToolName: "web_search", Operation: stringValue(action["type"]), InputChars: int64(len(input))}}
	case "tool_search_call":
		input := compactJSONValue(payload["arguments"])
		return []activityEvent{{Surface: "plugin", ToolName: "tool_search", InputChars: int64(len(input))}}
	}
	return nil
}

func summarizeActivityEvents(events []activityEvent) ActivitySummary {
	surfaces := map[string]*ActivityBreakdown{}
	tools := map[string]*ActivityBreakdown{}
	integrations := map[string]*ActivityBreakdown{}
	operations := map[string]*ActivityBreakdown{}
	summary := ActivitySummary{}
	for _, event := range events {
		if event.ToolName == "" && event.Surface == "" {
			continue
		}
		if event.Surface == "" {
			event.Surface = "unknown"
		}
		addActivity(&summary.Totals, event)
		addBreakdown(surfaces, event.Surface, ActivityBreakdown{Surface: event.Surface}, event)
		if event.ToolName != "" {
			addBreakdown(tools, event.ToolName, ActivityBreakdown{Surface: event.Surface, ToolName: event.ToolName}, event)
		}
		if event.Provider != "" {
			addBreakdown(integrations, event.Provider, ActivityBreakdown{Surface: event.Surface, Provider: event.Provider}, event)
		}
		operationKey := strings.Join([]string{event.Provider, event.Operation, event.ToolName}, "\x00")
		if event.Operation != "" || event.Provider != "" {
			addBreakdown(operations, operationKey, ActivityBreakdown{Surface: event.Surface, ToolName: event.ToolName, Provider: event.Provider, Operation: event.Operation}, event)
		}
	}
	summary.Surfaces = sortedActivityBreakdowns(surfaces)
	summary.Tools = sortedActivityBreakdowns(tools)
	summary.Integrations = sortedActivityBreakdowns(integrations)
	summary.Operations = sortedActivityBreakdowns(operations)
	return summary
}

func addActivity(total *ActivityTotals, event activityEvent) {
	total.Calls++
	if event.IsError {
		total.Errors++
	}
	total.InputChars += event.InputChars
	total.OutputChars += event.OutputChars
	total.ObservedTokens += event.ObservedTokens
	total.EstimatedTokens += estimateTokens(event.InputChars + event.OutputChars)
}

func addBreakdown(target map[string]*ActivityBreakdown, key string, initial ActivityBreakdown, event activityEvent) {
	entry := target[key]
	if entry == nil {
		copy := initial
		entry = &copy
		target[key] = entry
	}
	entry.Calls++
	if event.IsError {
		entry.Errors++
	}
	entry.InputChars += event.InputChars
	entry.OutputChars += event.OutputChars
	entry.ObservedTokens += event.ObservedTokens
	entry.EstimatedTokens += estimateTokens(event.InputChars + event.OutputChars)
}

func sortedActivityBreakdowns(values map[string]*ActivityBreakdown) []ActivityBreakdown {
	items := make([]ActivityBreakdown, 0, len(values))
	for _, value := range values {
		items = append(items, *value)
	}
	sort.Slice(items, func(i, j int) bool {
		if items[i].EstimatedTokens == items[j].EstimatedTokens {
			return items[i].Calls > items[j].Calls
		}
		return items[i].EstimatedTokens > items[j].EstimatedTokens
	})
	if len(items) > 12 {
		items = items[:12]
	}
	return items
}

func aggregateActivitySummaries(summaries []ActivitySummary, totalCost float64) ActivitySummary {
	surfaces := map[string]*ActivityBreakdown{}
	tools := map[string]*ActivityBreakdown{}
	integrations := map[string]*ActivityBreakdown{}
	operations := map[string]*ActivityBreakdown{}
	result := ActivitySummary{}
	for _, summary := range summaries {
		mergeActivityTotals(&result.Totals, summary.Totals)
		mergeBreakdownList(surfaces, summary.Surfaces, func(item ActivityBreakdown) string { return item.Surface })
		mergeBreakdownList(tools, summary.Tools, func(item ActivityBreakdown) string { return item.ToolName })
		mergeBreakdownList(integrations, summary.Integrations, func(item ActivityBreakdown) string { return item.Provider })
		mergeBreakdownList(operations, summary.Operations, func(item ActivityBreakdown) string {
			return strings.Join([]string{item.Provider, item.Operation, item.ToolName}, "\x00")
		})
	}
	result.Surfaces = sortedActivityBreakdowns(surfaces)
	result.Tools = sortedActivityBreakdowns(tools)
	result.Integrations = sortedActivityBreakdowns(integrations)
	result.Operations = sortedActivityBreakdowns(operations)
	assignActivityCost(&result, totalCost)
	return result
}

func mergeActivityTotals(target *ActivityTotals, source ActivityTotals) {
	target.Calls += source.Calls
	target.Errors += source.Errors
	target.InputChars += source.InputChars
	target.OutputChars += source.OutputChars
	target.EstimatedTokens += source.EstimatedTokens
	target.ObservedTokens += source.ObservedTokens
}

func mergeBreakdownList(target map[string]*ActivityBreakdown, source []ActivityBreakdown, keyFunc func(ActivityBreakdown) string) {
	for _, item := range source {
		key := keyFunc(item)
		if key == "" {
			continue
		}
		entry := target[key]
		if entry == nil {
			copy := item
			copy.EstimatedCost = 0
			target[key] = &copy
			continue
		}
		entry.Calls += item.Calls
		entry.Errors += item.Errors
		entry.InputChars += item.InputChars
		entry.OutputChars += item.OutputChars
		entry.EstimatedTokens += item.EstimatedTokens
		entry.ObservedTokens += item.ObservedTokens
	}
}

func loadPiDirectMcpToolMap() map[string]mcpDirectToolInfo {
	home, err := os.UserHomeDir()
	if err != nil {
		return nil
	}
	cachePath := filepath.Join(home, ".pi", "agent", "mcp-cache.json")
	bytes, err := os.ReadFile(cachePath)
	if err != nil {
		return nil
	}
	var cache struct {
		Servers map[string]struct {
			Tools []struct {
				Name string `json:"name"`
			} `json:"tools"`
			Resources []struct {
				Name string `json:"name"`
			} `json:"resources"`
		} `json:"servers"`
	}
	if err := json.Unmarshal(bytes, &cache); err != nil {
		return nil
	}
	mapping := map[string]mcpDirectToolInfo{}
	for server, entry := range cache.Servers {
		for _, tool := range entry.Tools {
			for _, name := range piMcpDirectToolNameCandidates(server, tool.Name) {
				mapping[name] = mcpDirectToolInfo{Server: server, Operation: tool.Name}
			}
		}
		for _, resource := range entry.Resources {
			operation := "get_" + normalizeMcpToolName(resource.Name)
			for _, name := range piMcpDirectToolNameCandidates(server, operation) {
				mapping[name] = mcpDirectToolInfo{Server: server, Operation: operation}
			}
		}
	}
	return mapping
}

func piMcpDirectToolNameCandidates(serverName string, toolName string) []string {
	serverPrefix := normalizeMcpToolName(serverName)
	shortPrefix := normalizeMcpToolName(strings.TrimSuffix(strings.TrimSuffix(serverName, "-mcp"), "mcp"))
	if shortPrefix == "" {
		shortPrefix = "mcp"
	}
	candidates := []string{}
	if serverPrefix != "" {
		candidates = append(candidates, serverPrefix+"_"+toolName)
	}
	if shortPrefix != "" && shortPrefix != serverPrefix {
		candidates = append(candidates, shortPrefix+"_"+toolName)
	}
	return candidates
}

func normalizeMcpToolName(value string) string {
	return strings.ReplaceAll(value, "-", "_")
}

func applyPiDirectMcpInfo(event *activityEvent, directMcpTools map[string]mcpDirectToolInfo) {
	if len(directMcpTools) == 0 || event == nil {
		return
	}
	info, ok := directMcpTools[event.ToolName]
	if !ok {
		return
	}
	event.Surface = "mcp"
	event.Provider = info.Server
	event.Operation = info.Operation
}

func assignActivityCost(summary *ActivitySummary, totalCost float64) {
	if summary.Totals.EstimatedTokens <= 0 || totalCost <= 0 {
		return
	}
	assign := func(tokens int64) float64 {
		return totalCost * float64(tokens) / float64(summary.Totals.EstimatedTokens)
	}
	summary.Totals.EstimatedCost = totalCost
	for i := range summary.Surfaces {
		summary.Surfaces[i].EstimatedCost = assign(summary.Surfaces[i].EstimatedTokens)
	}
	for i := range summary.Tools {
		summary.Tools[i].EstimatedCost = assign(summary.Tools[i].EstimatedTokens)
	}
	for i := range summary.Integrations {
		summary.Integrations[i].EstimatedCost = assign(summary.Integrations[i].EstimatedTokens)
	}
	for i := range summary.Operations {
		summary.Operations[i].EstimatedCost = assign(summary.Operations[i].EstimatedTokens)
	}
}

func classifySurface(toolName string, args map[string]any) string {
	name := strings.ToLower(toolName)
	if name == "mcp" {
		return "mcp"
	}
	if strings.HasPrefix(name, "chrome_devtools") {
		return "browser"
	}
	if strings.HasPrefix(name, "mobile_mcp") || strings.HasPrefix(name, "mobile_") {
		return "tool"
	}
	if name == "web_search" || name == "fetch_content" || name == "code_search" {
		return "web"
	}
	if name == "subagent" {
		return "subagent"
	}
	if name == "bash" || name == "exec_command" || name == "write_stdin" || name == "grep" || name == "find" || name == "ls" {
		return "shell"
	}
	if name == "read" || name == "write" || name == "edit" || name == "apply_patch" {
		return "filesystem"
	}
	if strings.Contains(name, "prometheus") || strings.Contains(name, "loki") || strings.Contains(name, "sentry") || strings.Contains(name, "slack") {
		return "plugin"
	}
	return "tool"
}

func providerOperation(toolName string, args map[string]any) (string, string) {
	if toolName == "mcp" {
		return stringValue(args["server"]), stringValue(args["tool"])
	}
	name := strings.ToLower(toolName)
	if strings.HasPrefix(name, "chrome_devtools_") {
		return "chrome_devtools", strings.TrimPrefix(toolName, "chrome_devtools_")
	}

	if strings.Contains(name, "prometheus") || strings.Contains(name, "loki") {
		return "grafana", toolName
	}
	if strings.Contains(name, "sentry") {
		return "sentry", toolName
	}
	if strings.Contains(name, "slack") {
		return "slack", toolName
	}
	return "", ""
}

func parseJSONObject(text string) map[string]any {
	var out map[string]any
	_ = json.Unmarshal([]byte(text), &out)
	return out
}

func parseObservedTokens(text string) int64 {
	match := originalTokenCountPattern.FindStringSubmatch(text)
	if len(match) != 2 {
		return 0
	}
	var value int64
	_, _ = fmt.Sscan(match[1], &value)
	return value
}

func estimateTokens(chars int64) int64 {
	if chars <= 0 {
		return 0
	}
	return (chars + 3) / 4
}
