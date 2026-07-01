import {Component, Fragment, type ErrorInfo, type KeyboardEvent as ReactKeyboardEvent, type ReactNode, useEffect, useMemo, useRef, useState} from "react";
import {AssistantRuntimeProvider, useExternalStoreRuntime} from "@assistant-ui/react";
import {Thread} from "./components/assistant-ui/thread";
import ReactMarkdown from "react-markdown";
import remarkGfm from "remark-gfm";
import {Bar, BarChart, CartesianGrid, Cell, ResponsiveContainer, Tooltip, XAxis, YAxis} from "recharts";
import {
  Activity,
  BarChart3,
  CalendarDays,
  Clock3,
  Command,
  DollarSign,
  FolderGit2,
  FolderOpen,
  MessageSquare,
  Layers3,
  RefreshCw,
  Search,
  Settings2,
  Sparkles,
  X,
} from "lucide-react";
import "./App.css";
import {
  GetConfig,
  GetProjectIndex,
  GetReport,
  GetRunner,
  GetSessionConversation,
  GetSessionPreview,
  OpenPathInFinder,
  OpenProjectInFinder,
  RefreshProjectIndex,
  SaveConfig,
} from "../wailsjs/go/app/App";

type ReportKey = "daily" | "weekly" | "monthly" | "session" | "projects" | "settings";
type IndexGroupBy = "project" | "agent" | "model";
type SortField = "lastActivity" | "totalCost" | "totalTokens";
type UsageChartMetric = "totalCost" | "totalTokens" | "inputTokens" | "outputTokens" | "cacheReadTokens";
type ProjectChartMetric = "totalCost" | "totalTokens" | "sessionCount" | "cacheReadTokens";
type DatePreset = "all" | "today" | "7d" | "30d" | "month" | "custom";

type RunnerInfo = {
  name: string;
  path: string;
  args: string[] | null;
  available: boolean;
  message: string;
};

type ModelBreakdown = {
  modelName: string;
  inputTokens: number;
  outputTokens: number;
  cacheCreationTokens: number;
  cacheReadTokens: number;
  cost: number;
};

type ReportRow = {
  period: string;
  agent: string;
  inputTokens: number;
  outputTokens: number;
  cacheCreationTokens: number;
  cacheReadTokens: number;
  totalTokens: number;
  totalCost: number;
  modelsUsed: string[] | null;
  modelBreakdowns: ModelBreakdown[] | null;
  metadata: Record<string, unknown> | null;
};

type ReportResponse = {
  report: ReportKey;
  source: string;
  runner: RunnerInfo;
  command: string[];
  rows: ReportRow[];
  totals: Record<string, unknown>;
  generated: string;
};

type ActivityBreakdown = {
  surface?: string;
  toolName?: string;
  provider?: string;
  operation?: string;
  calls: number;
  errors: number;
  inputChars: number;
  outputChars: number;
  estimatedTokens: number;
  observedTokens: number;
  estimatedCost: number;
};

type ActivitySummary = {
  surfaces: ActivityBreakdown[] | null;
  tools: ActivityBreakdown[] | null;
  integrations: ActivityBreakdown[] | null;
  operations: ActivityBreakdown[] | null;
  totals: ActivityBreakdown;
};

type ProjectSummary = {
  projectPath: string;
  projectName: string;
  physicalPaths: string[] | null;
  pathExists: boolean;
  groupingRule: string;
  agents: string[] | null;
  sessionCount: number;
  lastActivity: string;
  inputTokens: number;
  outputTokens: number;
  cacheCreationTokens: number;
  cacheReadTokens: number;
  totalTokens: number;
  totalCost: number;
  modelBreakdowns: ModelBreakdown[] | null;
  activity: ActivitySummary;
  recentSessions: IndexedSession[] | null;
};

type IndexedSession = {
  sessionId: string;
  agent: string;
  projectPath: string;
  projectName: string;
  lastActivity: string;
  inputTokens: number;
  outputTokens: number;
  cacheCreationTokens: number;
  cacheReadTokens: number;
  totalTokens: number;
  totalCost: number;
  modelBreakdowns: ModelBreakdown[] | null;
  lastUserMessage: string;
  lastUserMessageAt: string;
  messageSourcePath: string;
  activeDurationSeconds: number;
  originator: string;
  clientSource: string;
  model: string;
  provider: string;
  reasoningLevel: string;
  activity: ActivitySummary;
};

type SessionPreviewResponse = {
  sessionId: string;
  agent: string;
  preview: string;
  timestamp: string;
  sourcePath: string;
  activeDurationSeconds: number;
  originator: string;
  clientSource: string;
  model: string;
  provider: string;
  reasoningLevel: string;
  cached: boolean;
  supported: boolean;
  unavailableHint: string;
};

type ConversationMessage = {
  id?: string;
  parentId?: string;
  role: "user" | "assistant" | string;
  type: string;
  timestamp: string;
  text: string;
  toolName?: string;
  toolCallId?: string;
  isError?: boolean;
  hiddenByDefault?: boolean;
};

type SessionConversationResponse = {
  sessionId: string;
  agent: string;
  projectPath: string;
  sourcePath: string;
  activeDurationSeconds: number;
  originator: string;
  clientSource: string;
  model: string;
  provider: string;
  reasoningLevel: string;
  messages: ConversationMessage[] | null;
  supported: boolean;
  unavailableHint: string;
};

class ErrorBoundary extends Component<{children: ReactNode}, {error: Error | null; errorInfo: ErrorInfo | null}> {
  state: {error: Error | null; errorInfo: ErrorInfo | null} = {error: null, errorInfo: null};

  static getDerivedStateFromError(error: Error) {
    return {error, errorInfo: null};
  }

  componentDidCatch(error: Error, errorInfo: ErrorInfo) {
    this.setState({error, errorInfo});
  }

  render() {
    if (!this.state.error) return this.props.children;
    return (
      <div className="fixed inset-0 z-[60] overflow-auto bg-app-bg p-6 text-app-text">
        <div className="rounded-lg border border-red-500/40 bg-red-950/20 p-4">
          <div className="mb-2 text-lg font-semibold text-red-200">Conversation render error</div>
          <pre className="whitespace-pre-wrap text-xs text-red-100">{this.state.error.stack || this.state.error.message}</pre>
          {this.state.errorInfo?.componentStack ? <pre className="mt-4 whitespace-pre-wrap text-xs text-app-muted">{this.state.errorInfo.componentStack}</pre> : null}
        </div>
      </div>
    );
  }
}

type IndexGroup = {
  name: string;
  groupBy: "agent" | "model";
  projectCount: number;
  sessionCount: number;
  lastActivity: string;
  inputTokens: number;
  outputTokens: number;
  cacheCreationTokens: number;
  cacheReadTokens: number;
  totalTokens: number;
  totalCost: number;
  agents: string[] | null;
  modelBreakdowns: ModelBreakdown[] | null;
};

type IndexedAggregate = Pick<
  ProjectSummary,
  "totalCost" | "totalTokens" | "inputTokens" | "outputTokens" | "cacheReadTokens" | "sessionCount"
>;

type ProjectIndexResponse = {
  projects: ProjectSummary[];
  agentGroups: IndexGroup[];
  modelGroups: IndexGroup[];
  database: string;
  lastIndexed: string;
  runner: RunnerInfo;
  command: string[] | null;
  generated: string;
};

type ReportDefinition = {
  key: ReportKey;
  label: string;
  description: string;
  icon: typeof CalendarDays;
};

const reports: ReportDefinition[] = [
  {key: "daily", label: "Daily", description: "Usage grouped by day", icon: CalendarDays},
  {key: "weekly", label: "Weekly", description: "Usage grouped by week", icon: BarChart3},
  {key: "monthly", label: "Monthly", description: "Usage grouped by month", icon: Layers3},
  {key: "session", label: "Sessions", description: "Conversation-level usage", icon: Activity},
  {key: "projects", label: "Projects", description: "Indexed usage by project", icon: FolderGit2},
  {key: "settings", label: "Settings", description: "Configure project grouping", icon: Settings2},
];

const sources = [
  "all",
  "claude",
  "codex",
  "opencode",
  "amp",
  "droid",
  "codebuff",
  "hermes",
  "pi",
  "goose",
  "kilo",
  "copilot",
  "gemini",
  "kimi",
  "qwen",
  "openclaw",
];

const pageSizeOptions = [10, 25, 50];
const defaultDatePreset: DatePreset = "7d";

function dateRangeForPreset(preset: DatePreset) {
  const today = new Date();
  const isoToday = today.toISOString().slice(0, 10);
  if (preset === "all") {
    return {since: "", until: ""};
  }
  if (preset === "today") {
    return {since: isoToday, until: isoToday};
  }
  if (preset === "7d" || preset === "30d") {
    const days = preset === "7d" ? 7 : 30;
    return {since: new Date(Date.now() - 1000 * 60 * 60 * 24 * days).toISOString().slice(0, 10), until: ""};
  }
  if (preset === "month") {
    return {since: new Date(today.getFullYear(), today.getMonth(), 1).toISOString().slice(0, 10), until: ""};
  }
  return {
    since: localStorage.getItem("ccusage-ui.since") ?? "",
    until: localStorage.getItem("ccusage-ui.until") ?? "",
  };
}

function initialDatePreset() {
  const stored = localStorage.getItem("ccusage-ui.datePreset") as DatePreset | null;
  return stored && ["all", "today", "7d", "30d", "month", "custom"].includes(stored) ? stored : defaultDatePreset;
}

function initialSource() {
  const stored = localStorage.getItem("ccusage-ui.source");
  return stored && sources.includes(stored) ? stored : "all";
}

function App() {
  const initialPreset = initialDatePreset();
  const initialRange = dateRangeForPreset(initialPreset);
  const [report, setReport] = useState<ReportKey>("projects");
  const [source, setSource] = useState(initialSource);
  const [since, setSince] = useState(initialRange.since);
  const [until, setUntil] = useState(initialRange.until);
  const [datePreset, setDatePreset] = useState<DatePreset>(initialPreset);
  const [offline, setOffline] = useState(false);
  const [noCost, setNoCost] = useState(false);
  const [query, setQuery] = useState("");
  const [indexGroupBy, setIndexGroupBy] = useState<IndexGroupBy>("project");
  const [projectSort, setProjectSort] = useState<SortField>("lastActivity");
  const [usageChartMetric, setUsageChartMetric] = useState<UsageChartMetric>("totalCost");
  const [projectChartMetric, setProjectChartMetric] = useState<ProjectChartMetric>("totalCost");
  const [selectedIndex, setSelectedIndex] = useState(0);
  const [runner, setRunner] = useState<RunnerInfo | null>(null);
  const [data, setData] = useState<ReportResponse | null>(null);
  const [projectIndex, setProjectIndex] = useState<ProjectIndexResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [initialIndexing, setInitialIndexing] = useState(false);
  const [error, setError] = useState("");
  const [conversationSession, setConversationSession] = useState<IndexedSession | null>(null);
  const [sessionDetail, setSessionDetail] = useState<IndexedSession | null>(null);
  const [conversation, setConversation] = useState<SessionConversationResponse | null>(null);
  const [conversationLoading, setConversationLoading] = useState(false);
  const [conversationError, setConversationError] = useState("");
  const [configJson, setConfigJson] = useState("");
  const [settingsStatus, setSettingsStatus] = useState("");
  const [sessionPreviewCache, setSessionPreviewCache] = useState<Record<string, SessionPreviewResponse>>({});

  useEffect(() => {
    GetRunner().then(setRunner).catch((err) => setError(errorMessage(err)));
  }, []);

  useEffect(() => {
    void loadReport();
  }, [report, source, since, until, offline, noCost]);

  useEffect(() => {
    localStorage.setItem("ccusage-ui.source", source);
  }, [source]);

  useEffect(() => {
    localStorage.setItem("ccusage-ui.datePreset", datePreset);
    if (datePreset === "custom") {
      localStorage.setItem("ccusage-ui.since", since);
      localStorage.setItem("ccusage-ui.until", until);
    }
  }, [datePreset, since, until]);

  const filteredProjects = useMemo(() => {
    const projects = projectIndex?.projects ?? [];
    const trimmed = query.trim().toLowerCase();
    const filtered = !trimmed
      ? projects
      : projects.filter((project) => {
          const haystack = [
            project.projectName,
            project.projectPath,
            project.agents?.join(" "),
            project.modelBreakdowns?.map((model) => model.modelName).join(" "),
          ]
            .join(" ")
            .toLowerCase();
          return haystack.includes(trimmed);
        });

    return sortUsageItems(filtered, projectSort);
  }, [projectIndex, projectSort, query]);

  const filteredIndexGroups = useMemo(() => {
    const groups = indexGroupBy === "agent" ? projectIndex?.agentGroups ?? [] : projectIndex?.modelGroups ?? [];
    const trimmed = query.trim().toLowerCase();
    const filtered = !trimmed
      ? groups
      : groups.filter((group) => {
          const haystack = [
            group.name,
            group.groupBy,
            group.agents?.join(" "),
            group.modelBreakdowns?.map((model) => model.modelName).join(" "),
          ]
            .join(" ")
            .toLowerCase();
          return haystack.includes(trimmed);
        });

    return sortUsageItems(filtered, projectSort);
  }, [indexGroupBy, projectIndex, projectSort, query]);

  const filteredRows = useMemo(() => {
    const rows = data?.rows ?? [];
    const trimmed = query.trim().toLowerCase();
    if (!trimmed) {
      return rows;
    }

    return rows.filter((row) => {
      const haystack = [
        row.period,
        row.agent,
        row.modelsUsed?.join(" "),
        String(row.metadata?.projectPath ?? ""),
        String(row.metadata?.lastActivity ?? ""),
      ]
        .join(" ")
        .toLowerCase();
      return haystack.includes(trimmed);
    });
  }, [data, query]);

  const selectedRow = filteredRows[Math.min(selectedIndex, Math.max(filteredRows.length - 1, 0))];
  const selectedProject = filteredProjects[Math.min(selectedIndex, Math.max(filteredProjects.length - 1, 0))];
  const selectedIndexGroup = filteredIndexGroups[Math.min(selectedIndex, Math.max(filteredIndexGroups.length - 1, 0))];
  const activeSelectedRow = report !== "projects" ? selectedRow : undefined;
  const activeSelectedProject = report === "projects" && indexGroupBy === "project" ? selectedProject : undefined;
  const activeSelectedIndexGroup = report === "projects" && indexGroupBy !== "project" ? selectedIndexGroup : undefined;
  const showTopOverview = !(report === "projects" && activeSelectedProject);

  const totals = useMemo(() => {
    if (report === "projects") {
      const sourceRows: IndexedAggregate[] = indexGroupBy === "project" ? filteredProjects : filteredIndexGroups;
      return sourceRows.reduce(
        (acc, item) => ({
          totalCost: acc.totalCost + item.totalCost,
          totalTokens: acc.totalTokens + item.totalTokens,
          inputTokens: acc.inputTokens + item.inputTokens,
          outputTokens: acc.outputTokens + item.outputTokens,
          cacheReadTokens: acc.cacheReadTokens + item.cacheReadTokens,
        }),
        {totalCost: 0, totalTokens: 0, inputTokens: 0, outputTokens: 0, cacheReadTokens: 0},
      );
    }

    if (data?.totals && Object.keys(data.totals).length > 0) {
      return {
        totalCost: toNumber(data.totals.totalCost),
        totalTokens: toNumber(data.totals.totalTokens),
        inputTokens: toNumber(data.totals.inputTokens),
        outputTokens: toNumber(data.totals.outputTokens),
        cacheReadTokens: toNumber(data.totals.cacheReadTokens),
      };
    }

    return filteredRows.reduce(
      (acc, row) => ({
        totalCost: acc.totalCost + row.totalCost,
        totalTokens: acc.totalTokens + row.totalTokens,
        inputTokens: acc.inputTokens + row.inputTokens,
        outputTokens: acc.outputTokens + row.outputTokens,
        cacheReadTokens: acc.cacheReadTokens + row.cacheReadTokens,
      }),
      {totalCost: 0, totalTokens: 0, inputTokens: 0, outputTokens: 0, cacheReadTokens: 0},
    );
  }, [data, filteredIndexGroups, filteredProjects, filteredRows, indexGroupBy, report]);

  async function loadReport() {
    if (report === "settings") {
      await loadSettings();
      return;
    }
    if (report === "projects") {
      await loadProjectIndex(false);
      return;
    }

    setInitialIndexing(false);
    setLoading(true);
    setError("");
    try {
      const response = await GetReport({
        report,
        source,
        since,
        until: ccusageUntilDate(until),
        offline,
        noCost,
      });
      setData(response as ReportResponse);
      setSelectedIndex(0);
      setRunner((response as ReportResponse).runner);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }

  async function loadProjectIndex(refresh: boolean) {
    setLoading(true);
    setInitialIndexing(false);
    setError("");
    try {
      const request = {
        source,
        since,
        until: ccusageUntilDate(until),
        offline,
        noCost,
      };
      let response = (refresh ? await RefreshProjectIndex(request) : await GetProjectIndex(request)) as ProjectIndexResponse;
      if (!refresh && !response.lastIndexed) {
        setInitialIndexing(true);
        response = (await RefreshProjectIndex(request)) as ProjectIndexResponse;
      }
      setProjectIndex(response);
      setSelectedIndex(0);
      setRunner(response.runner);
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
      setInitialIndexing(false);
    }
  }

  async function loadSettings() {
    setInitialIndexing(false);
    setLoading(true);
    setError("");
    setSettingsStatus("");
    try {
      const config = await GetConfig();
      setConfigJson(JSON.stringify(config, null, 2));
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }

  async function saveSettings() {
    setLoading(true);
    setError("");
    setSettingsStatus("");
    try {
      const parsed = JSON.parse(configJson);
      const saved = await SaveConfig(parsed);
      setConfigJson(JSON.stringify(saved, null, 2));
      setSettingsStatus("Settings saved. Refresh the Projects index to apply grouping changes.");
    } catch (err) {
      setError(errorMessage(err));
    } finally {
      setLoading(false);
    }
  }

  function refreshCurrentView() {
    if (report === "settings") {
      void loadSettings();
      return;
    }
    if (report === "projects") {
      void loadProjectIndex(true);
      return;
    }
    void loadReport();
  }

  async function openConversation(session: IndexedSession) {
    setConversationSession(session);
    setConversation(null);
    setConversationError("");
    setConversationLoading(true);
    try {
      const response = await GetSessionConversation({
        agent: session.agent,
        sessionId: session.sessionId,
        projectPath: session.projectPath,
      });
      const typed = response as SessionConversationResponse;
      setConversation(typed);
      setSessionPreviewCache((current) => ({
        ...current,
        [sessionPreviewKey(session)]: {
          ...(current[sessionPreviewKey(session)] ?? {}),
          sessionId: session.sessionId,
          agent: session.agent,
          preview: current[sessionPreviewKey(session)]?.preview ?? session.lastUserMessage ?? "",
          timestamp: current[sessionPreviewKey(session)]?.timestamp ?? session.lastUserMessageAt ?? "",
          sourcePath: typed.sourcePath || current[sessionPreviewKey(session)]?.sourcePath || session.messageSourcePath || "",
          activeDurationSeconds: typed.activeDurationSeconds,
          originator: typed.originator,
          clientSource: typed.clientSource,
          model: typed.model,
          provider: typed.provider,
          reasoningLevel: typed.reasoningLevel,
          cached: true,
          supported: typed.supported,
          unavailableHint: typed.unavailableHint,
        },
      }));
    } catch (err) {
      setConversationError(errorMessage(err));
    } finally {
      setConversationLoading(false);
    }
  }

  function cacheSessionPreview(session: IndexedSession, preview: SessionPreviewResponse) {
    setSessionPreviewCache((current) => ({
      ...current,
      [sessionPreviewKey(session)]: preview,
    }));
  }

  function closeConversation() {
    setConversationSession(null);
    setConversation(null);
    setConversationError("");
    setConversationLoading(false);
  }

  function applyDatePreset(nextPreset: DatePreset) {
    setDatePreset(nextPreset);
    const range = dateRangeForPreset(nextPreset);
    setSince(range.since);
    setUntil(range.until);
  }

  function selectReport(nextReport: ReportKey) {
    setReport(nextReport);
    setSelectedIndex(0);
    if (nextReport === "settings") {
      return;
    }
  }

  return (
    <>
    <main className="h-screen overflow-hidden bg-app-bg text-app-text">
      <div className="grid h-full grid-cols-[220px_360px_minmax(0,1fr)]">
        <aside className="flex min-h-0 flex-col border-r border-app-line bg-app-sidebar/95">
          <div className="px-5 pb-4 pt-5">
            <div className="flex items-center gap-2 text-[17px] font-semibold tracking-normal">
              <span className="grid h-7 w-7 place-items-center rounded-md bg-app-accent text-white">
                <Sparkles size={16} strokeWidth={2.4} />
              </span>
              ccusage
            </div>
            <div className="mt-3 text-xs leading-5 text-app-muted">
              {runner?.available ? runner.message : "Detecting runner..."}
            </div>
          </div>

          <nav className="flex-1 space-y-1 px-3">
            {reports.map((item) => {
              const Icon = item.icon;
              const active = item.key === report;
              return (
                <button
                  key={item.key}
                  onClick={() => selectReport(item.key)}
                  className={[
                    "group flex w-full items-center gap-3 rounded-md px-3 py-2 text-left text-sm transition",
                    active
                      ? "bg-app-accentSoft text-app-text"
                      : "text-app-muted hover:bg-app-panel hover:text-app-text",
                  ].join(" ")}
                >
                  <Icon size={17} />
                  <span>{item.label}</span>
                </button>
              );
            })}
          </nav>

          <div className="border-t border-app-line px-4 py-4">
            {report === "projects" ? (
              <p className="text-xs leading-5 text-app-muted">
                Last indexed {projectIndex?.lastIndexed ? formatDateTime(projectIndex.lastIndexed) : "—"}
              </p>
            ) : (
              <>
                <div className="flex items-center gap-2 text-xs font-medium text-app-muted">
                  <Settings2 size={15} />
                  Direct refresh
                </div>
                <p className="mt-2 text-xs leading-5 text-app-muted">
                  Results come straight from ccusage. No persistent cache.
                </p>
              </>
            )}
          </div>
        </aside>

        <section className="flex min-h-0 flex-col border-r border-app-line bg-app-panel">
          <div className="border-b border-app-line px-4 py-4">
            <div className="mb-3 flex items-center justify-between gap-3">
              <div>
                <h1 className="text-lg font-semibold">{activeReport(report).label}</h1>
                <p className="mt-0.5 text-xs text-app-muted">{activeReport(report).description}</p>
              </div>
              {report === "projects" ? (
                <button
                  className="button shrink-0"
                  onClick={refreshCurrentView}
                  disabled={loading}
                  title="Refresh Projects index"
                >
                  <RefreshCw size={16} className={loading ? "animate-spin" : ""} />
                  {loading ? "Refreshing…" : "Refresh Index"}
                </button>
              ) : (
                <button
                  className="icon-button"
                  onClick={refreshCurrentView}
                  disabled={loading}
                  title="Refresh"
                >
                  <RefreshCw size={16} className={loading ? "animate-spin" : ""} />
                </button>
              )}
            </div>

            {report !== "settings" ? (
              <>
                <label className="relative block">
                  <Search className="pointer-events-none absolute left-3 top-2.5 text-app-muted" size={15} />
                  <input
                    value={query}
                    onChange={(event) => setQuery(event.target.value)}
                    className="control h-9 w-full pl-9"
                    placeholder="Search"
                  />
                </label>

                <div className="mt-3 grid grid-cols-2 gap-2">
                  <select className="control h-9" value={source} onChange={(event) => setSource(event.target.value)}>
                    {sources.map((item) => (
                      <option key={item} value={item}>
                        {item === "all" ? "All sources" : item}
                      </option>
                    ))}
                  </select>
                  <select className="control h-9" value={datePreset} onChange={(event) => applyDatePreset(event.target.value as DatePreset)}>
                    <option value="all">All time</option>
                    <option value="today">Today</option>
                    <option value="7d">Last 7 days</option>
                    <option value="30d">Last 30 days</option>
                    <option value="month">This month</option>
                    <option value="custom">Custom range</option>
                  </select>
                </div>

                {datePreset === "custom" ? (
                  <div className="mt-2 grid grid-cols-2 gap-2">
                    <input
                      className="control h-9"
                      type="date"
                      value={since}
                      onChange={(event) => setSince(event.target.value)}
                    />
                    <input
                      className="control h-9"
                      type="date"
                      value={until}
                      onChange={(event) => setUntil(event.target.value)}
                    />
                  </div>
                ) : null}

                <div className="mt-3 space-y-2">
                  {report === "projects" ? (
                    <>
                    <div className="flex rounded-md border border-app-line bg-app-surface p-0.5">
                      {(["project", "agent", "model"] as IndexGroupBy[]).map((item) => (
                        <button
                          key={item}
                          className={[
                            "h-7 rounded px-2 text-xs font-medium capitalize transition",
                            indexGroupBy === item ? "bg-app-accentSoft text-app-text" : "text-app-muted hover:text-app-text",
                          ].join(" ")}
                          onClick={() => {
                            setIndexGroupBy(item);
                            setSelectedIndex(0);
                          }}
                        >
                          {item}
                        </button>
                      ))}
                    </div>
                    <label className="flex items-center gap-2 text-xs text-app-muted">
                      Sort
                      <select
                        className="control h-8 flex-1"
                        value={projectSort}
                        onChange={(event) => setProjectSort(event.target.value as SortField)}
                      >
                        <option value="lastActivity">Last activity</option>
                        <option value="totalCost">Cost</option>
                        <option value="totalTokens">Total tokens</option>
                      </select>
                    </label>
                    </>
                  ) : null}


                </div>
              </>
            ) : null}
          </div>

          <div className="min-h-0 flex-1 overflow-y-auto">
            {error ? <ErrorState message={error} /> : null}
            {!error && report === "settings" ? (
              <EmptyState message="Edit the JSON settings on the right. Save, then refresh the Projects index." />
            ) : null}
            {!error && report === "projects" && initialIndexing ? (
              <EmptyState message="Building your Projects index for the first time. This can take a while for large ccusage histories — please keep the app open." />
            ) : null}
            {!error && report === "projects" && indexGroupBy === "project" && filteredProjects.length === 0 && !loading ? (
              <EmptyState
                message={projectIndex?.lastIndexed ? "No projects match the current source, date, or search filters." : "No indexed projects yet. The app will build the index automatically on first load."}
                actionLabel="Reload Projects"
                onAction={() => void loadProjectIndex(false)}
              />
            ) : null}
            {!error && report === "projects" && indexGroupBy !== "project" && filteredIndexGroups.length === 0 && !loading ? (
              <EmptyState
                message={projectIndex?.lastIndexed ? "No groups match the current source, date, or search filters." : "No indexed groups yet. The app will build the index automatically on first load."}
                actionLabel="Reload Groups"
                onAction={() => void loadProjectIndex(false)}
              />
            ) : null}
            {!error && report !== "projects" && report !== "settings" && filteredRows.length === 0 && !loading ? (
              <EmptyState actionLabel="Refresh" onAction={refreshCurrentView} />
            ) : null}
            {!error && report === "projects" && indexGroupBy === "project"
              ? filteredProjects.map((project, index) => (
                  <button
                    key={`${project.projectPath}-${index}`}
                    onClick={() => setSelectedIndex(index)}
                    className={[
                      "flex w-full items-center gap-3 border-b border-app-line/75 px-4 py-3 text-left transition",
                      index === selectedIndex ? "bg-app-accentSoft/75" : "hover:bg-app-surface",
                    ].join(" ")}
                  >
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm font-medium">{project.projectName}</div>
                      <div className="mt-1.5 flex min-w-0 items-center gap-2 text-xs text-app-muted">
                        <AgentNameChips agents={project.agents ?? ["unknown"]} maxVisible={2} />
                        <span className="h-1 w-1 rounded-full bg-app-muted/50" />
                        <span>{project.sessionCount} sessions</span>
                      </div>
                    </div>
                    <div className="text-right">
                      <div className="text-sm font-semibold">{formatCost(project.totalCost, noCost)}</div>
                      <div className="mt-1 text-xs text-app-muted">{formatTokens(project.totalTokens)}</div>
                    </div>
                  </button>
                ))
              : null}
            {!error && report === "projects" && indexGroupBy !== "project"
              ? filteredIndexGroups.map((group, index) => (
                  <button
                    key={`${group.groupBy}-${group.name}-${index}`}
                    onClick={() => setSelectedIndex(index)}
                    className={[
                      "flex w-full items-center gap-3 border-b border-app-line/75 px-4 py-3 text-left transition",
                      index === selectedIndex ? "bg-app-accentSoft/75" : "hover:bg-app-surface",
                    ].join(" ")}
                  >
                    <div className="min-w-0 flex-1">
                      <div className="truncate text-sm font-medium">{group.name}</div>
                      <div className="mt-1.5 flex min-w-0 items-center gap-2 text-xs text-app-muted">
                        <AgentNameChips agents={group.agents ?? ["unknown"]} maxVisible={2} />
                        <span className="h-1 w-1 rounded-full bg-app-muted/50" />
                        <span>{group.projectCount} projects</span>
                      </div>
                    </div>
                    <div className="text-right">
                      <div className="text-sm font-semibold">{formatCost(group.totalCost, noCost)}</div>
                      <div className="mt-1 text-xs text-app-muted">{formatTokens(group.totalTokens)}</div>
                    </div>
                  </button>
                ))
              : null}
            {!error && report !== "projects" && report !== "settings" &&
              filteredRows.map((row, index) => (
                <button
                  key={`${row.period}-${index}`}
                  onClick={() => setSelectedIndex(index)}
                  className={[
                    "flex w-full items-center gap-3 border-b border-app-line/75 px-4 py-3 text-left transition",
                    index === selectedIndex ? "bg-app-accentSoft/75" : "hover:bg-app-surface",
                  ].join(" ")}
                >
                  <div className="min-w-0 flex-1">
                    <div className="truncate text-sm font-medium">{rowTitle(row, report)}</div>
                    <div className="mt-1.5 flex min-w-0 items-center gap-2 text-xs text-app-muted">
                      <AgentChips row={row} />
                      <span className="h-1 w-1 rounded-full bg-app-muted/50" />
                      <span>{formatTokens(row.totalTokens)}</span>
                    </div>
                  </div>
                  <div className="text-right">
                    <div className="text-sm font-semibold">{formatCost(row.totalCost, noCost)}</div>
                    <div className="mt-1 text-xs text-app-muted">{modelLabel(row)}</div>
                  </div>
                </button>
              ))}
          </div>
        </section>

        <section className="min-h-0 overflow-y-auto bg-app-bg">
          {report === "settings" ? (
            <SettingsPanel
              configJson={configJson}
              status={settingsStatus}
              loading={loading}
              onChange={setConfigJson}
              onSave={() => void saveSettings()}
              onReload={() => void loadSettings()}
            />
          ) : (
            <>
          {showTopOverview ? (
            <div className="border-b border-app-line bg-app-bg/95 px-6 py-4">
              {(report === "daily" || report === "weekly" || report === "monthly") && filteredRows.length > 0 ? (
                <UsageTrendChart
                  rows={filteredRows}
                  report={report}
                  metric={usageChartMetric}
                  selectedIndex={selectedIndex}
                  noCost={noCost}
                  onMetricChange={setUsageChartMetric}
                  onSelect={setSelectedIndex}
                />
              ) : null}
              {report === "projects" && (indexGroupBy === "project" ? filteredProjects.length > 0 : filteredIndexGroups.length > 0) ? (
                <ProjectUsageChart
                  rows={indexGroupBy === "project" ? filteredProjects : filteredIndexGroups}
                  groupBy={indexGroupBy}
                  metric={projectChartMetric}
                  selectedIndex={selectedIndex}
                  noCost={noCost}
                  onMetricChange={setProjectChartMetric}
                  onSelect={setSelectedIndex}
                />
              ) : null}
              <div className={(report === "daily" || report === "weekly" || report === "monthly" || report === "projects") && (filteredRows.length > 0 || filteredProjects.length > 0 || filteredIndexGroups.length > 0) ? "mt-4 grid grid-cols-4 gap-3" : "grid grid-cols-4 gap-3"}>
                <Metric icon={DollarSign} label="Cost" value={formatCost(totals.totalCost, noCost)} />
                <Metric icon={Command} label="Tokens" value={formatTokens(totals.totalTokens)} />
                <Metric icon={Clock3} label="Input" value={formatTokens(totals.inputTokens)} />
                <Metric icon={Activity} label="Output" value={formatTokens(totals.outputTokens)} />
              </div>
            </div>
          ) : null}

          {loading && !activeSelectedRow && !activeSelectedProject && !activeSelectedIndexGroup ? (
            <div className="grid h-full place-items-center px-6 text-center text-app-muted">
              <div>
                <RefreshCw className="mx-auto mb-3 animate-spin text-app-accent" size={28} />
                <p className="text-sm font-medium text-app-text">
                  {report === "projects" ? (initialIndexing ? "Building your Projects index…" : "Refreshing Projects index…") : "Loading report…"}
                </p>
                <p className="mt-1 text-xs">
                  {report === "projects" ? "This can take a while for large ccusage histories." : "Fetching usage details from ccusage."}
                </p>
              </div>
            </div>
          ) : activeSelectedRow || activeSelectedProject || activeSelectedIndexGroup ? (
            <div className="px-5 py-4">
              <div className="mb-4 flex items-start justify-between gap-5">
                <div className="min-w-0">
                  <h2 className="truncate text-2xl font-semibold">
                    {activeSelectedProject
                      ? activeSelectedProject.projectName
                      : activeSelectedIndexGroup
                        ? activeSelectedIndexGroup.name
                        : activeSelectedRow
                          ? rowTitle(activeSelectedRow, report)
                          : ""}
                  </h2>
                  {activeSelectedProject ? null : activeSelectedIndexGroup ? (
                    <p className="mt-2 text-sm text-app-muted">
                      Grouped by {activeSelectedIndexGroup.groupBy} · {activeSelectedIndexGroup.projectCount} projects
                    </p>
                  ) : (
                    <p className="mt-2 text-sm text-app-muted">
                      {activeSelectedRow?.agent || "all"} · {activeSelectedRow?.modelsUsed?.join(", ") || "No model data"}
                    </p>
                  )}
                </div>
              </div>

              {activeSelectedProject ? (
                <>
                  <ProjectDetail
                    project={activeSelectedProject}
                    noCost={noCost}
                    database={projectIndex?.database ?? ""}
                    onOpenPath={(path) => void OpenProjectInFinder(path).catch((err) => setError(errorMessage(err)))}
                    previewCache={sessionPreviewCache}
                    onPreviewLoaded={cacheSessionPreview}
                    onOpenSession={(session) => void openConversation(session)}
                    onOpenSessionDetail={setSessionDetail}
                  />

                  <ProjectOverviewSection
                    rows={filteredProjects}
                    groupBy={indexGroupBy}
                    metric={projectChartMetric}
                    selectedIndex={selectedIndex}
                    totals={totals}
                    noCost={noCost}
                    onMetricChange={setProjectChartMetric}
                    onSelect={setSelectedIndex}
                  />
                </>
              ) : null}

              {activeSelectedIndexGroup ? (
                <IndexGroupDetail group={activeSelectedIndexGroup} noCost={noCost} database={projectIndex?.database ?? ""} />
              ) : null}

              {activeSelectedRow ? (
                <>
                  <div className="grid grid-cols-4 gap-3">
                    <DetailStat label="Cost" value={formatCost(activeSelectedRow.totalCost, noCost)} />
                    <DetailStat label="Total tokens" value={formatTokens(activeSelectedRow.totalTokens)} />
                    <DetailStat label="Cache read" value={formatTokens(activeSelectedRow.cacheReadTokens)} />
                    <DetailStat label="Cache create" value={formatTokens(activeSelectedRow.cacheCreationTokens)} />
                  </div>

                  <ModelBreakdownTable models={activeSelectedRow.modelBreakdowns ?? []} noCost={noCost} />


                </>
              ) : null}
            </div>
          ) : (
            <div className="grid h-full place-items-center px-6 text-center text-app-muted">
              <div>
                <Command className="mx-auto mb-3" size={28} />
                <p className="text-sm">Run a report to see usage details.</p>
              </div>
            </div>
          )}
            </>
          )}
        </section>
      </div>
    </main>
    {sessionDetail ? (
      <SessionDetailModal
        session={sessionDetail}
        noCost={noCost}
        onClose={() => setSessionDetail(null)}
        onOpenSession={(session) => void openConversation(session)}
        onOpenPath={(path) => void OpenPathInFinder(path).catch((err) => setError(errorMessage(err)))}
      />
    ) : null}
    {conversationSession ? (
      <ErrorBoundary>
        <ConversationModal
          session={conversationSession}
          conversation={conversation}
          loading={conversationLoading}
          error={conversationError}
          onClose={closeConversation}
          onOpenSource={(path) => void OpenPathInFinder(path).catch((err) => setError(errorMessage(err)))}
        />
      </ErrorBoundary>
    ) : null}
    </>
  );
}

function SettingsPanel({
  configJson,
  status,
  loading,
  onChange,
  onSave,
  onReload,
}: {
  configJson: string;
  status: string;
  loading: boolean;
  onChange: (value: string) => void;
  onSave: () => void;
  onReload: () => void;
}) {
  return (
    <div className="flex h-full flex-col px-6 py-5">
      <div className="mb-5 flex items-start justify-between gap-4">
        <div>
          <h2 className="text-2xl font-semibold">Settings</h2>
          <p className="mt-2 text-sm text-app-muted">
            Edit JSON settings directly. Project grouping rules are string-based and work even when worktree folders were deleted.
          </p>
        </div>
        <div className="flex gap-2">
          <button className="button" onClick={onReload} disabled={loading}>
            <RefreshCw size={15} className={loading ? "animate-spin" : ""} />
            Reload
          </button>
          <button className="button" onClick={onSave} disabled={loading}>
            Save
          </button>
        </div>
      </div>

      {status ? <div className="mb-4 rounded-md border border-app-line bg-app-surface px-3 py-2 text-sm text-app-muted">{status}</div> : null}

      <textarea
        className="control min-h-[520px] flex-1 resize-none p-4 font-mono text-xs leading-5"
        value={configJson}
        onChange={(event) => onChange(event.target.value)}
        onKeyDown={(event) => handleSettingsEditorKeyDown(event, configJson, onChange, onSave)}
        spellCheck={false}
      />

      <div className="mt-4 rounded-md border border-app-line bg-app-surface px-4 py-3 text-xs leading-5 text-app-muted">
        <div className="font-semibold text-app-text">Rule syntax</div>
        <div>
          Rules use <code>matchPath</code>, <code>groupAs</code>, and optional <code>displayAs</code>.
        </div>
        <div>{"{name}"} captures one folder. {"{name...}"} captures multiple folders until the next fixed folder matches. {"{home}"} expands to your home directory.</div>
        <div className="mt-2">
          Example display rule: <code>{`"displayAs": "lh-data-mesh/{domain}"`}</code>. After saving, refresh the Projects index.
        </div>
      </div>
    </div>
  );
}

function handleSettingsEditorKeyDown(
  event: ReactKeyboardEvent<HTMLTextAreaElement>,
  value: string,
  onChange: (value: string) => void,
  onSave: () => void,
) {
  if ((event.metaKey || event.ctrlKey) && event.key.toLowerCase() === "s") {
    event.preventDefault();
    onSave();
    return;
  }

  if (event.key !== "Tab") {
    return;
  }

  event.preventDefault();
  const textarea = event.currentTarget;
  const start = textarea.selectionStart;
  const end = textarea.selectionEnd;
  const indent = "  ";

  if (event.shiftKey) {
    const lineStart = value.lastIndexOf("\n", start - 1) + 1;
    const selectedText = value.slice(lineStart, end);
    const unindented = selectedText.replace(/^ {1,2}/gm, "");
    const nextValue = value.slice(0, lineStart) + unindented + value.slice(end);
    const removedBeforeStart = selectedText.slice(0, start - lineStart).length - unindented.slice(0, start - lineStart).length;
    const removedTotal = selectedText.length - unindented.length;

    onChange(nextValue);
    requestAnimationFrame(() => {
      textarea.selectionStart = Math.max(lineStart, start - removedBeforeStart);
      textarea.selectionEnd = Math.max(textarea.selectionStart, end - removedTotal);
    });
    return;
  }

  if (start !== end && value.slice(start, end).includes("\n")) {
    const lineStart = value.lastIndexOf("\n", start - 1) + 1;
    const selectedText = value.slice(lineStart, end);
    const indented = selectedText.replace(/^/gm, indent);
    const nextValue = value.slice(0, lineStart) + indented + value.slice(end);
    onChange(nextValue);
    requestAnimationFrame(() => {
      textarea.selectionStart = start + indent.length;
      textarea.selectionEnd = end + indented.length - selectedText.length;
    });
    return;
  }

  const nextValue = value.slice(0, start) + indent + value.slice(end);
  onChange(nextValue);
  requestAnimationFrame(() => {
    textarea.selectionStart = start + indent.length;
    textarea.selectionEnd = start + indent.length;
  });
}

function ConversationModal({
  session,
  conversation,
  loading,
  error,
  onClose,
  onOpenSource,
}: {
  session: IndexedSession;
  conversation: SessionConversationResponse | null;
  loading: boolean;
  error: string;
  onClose: () => void;
  onOpenSource: (path: string) => void;
}) {
  const messages = conversation?.messages ?? [];
  const [showFullTrace, setShowFullTrace] = useState(false);
  const [showUsage, setShowUsage] = useState(false);
  const visibleMessages = showFullTrace ? messages : messages.filter((message) => !message.hiddenByDefault || message.type === "toolCall");
  const hiddenMessageCount = messages.length - visibleMessages.length;

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        onClose();
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 px-6 py-6" onClick={onClose}>
      <div className="flex h-full max-h-full min-h-0 w-full max-w-5xl flex-col overflow-hidden rounded-lg border border-app-line bg-app-bg shadow-2xl" onClick={(event) => event.stopPropagation()}>
        <div className="flex items-start justify-between gap-4 border-b border-app-line bg-app-panel px-5 py-4">
          <div className="min-w-0">
            <div className="flex items-center gap-2 text-lg font-semibold">
              <MessageSquare size={18} />
              Conversation
            </div>
            <div className="mt-1 truncate text-xs text-app-muted">
              {session.agent}{conversationClientLabel(conversation, session) ? ` · ${conversationClientLabel(conversation, session)}` : ""}{conversationModelLabel(conversation, session) ? ` · ${conversationModelLabel(conversation, session)}` : ""} · {shortSessionId(session.sessionId)} · Duration {formatDuration(conversation?.activeDurationSeconds || session.activeDurationSeconds)} · {cleanProjectPath(session.projectPath)}
            </div>
          </div>
          <div className="flex items-center gap-2">
            {conversation?.sourcePath ? (
              <button className="button" onClick={() => onOpenSource(conversation.sourcePath)}>
                <FolderOpen size={15} />
                Reveal transcript
              </button>
            ) : null}
            <button className="icon-button" onClick={onClose} title="Close">
              <X size={17} />
            </button>
          </div>
        </div>

        {messages.length > 0 ? (
          <div className="flex items-center justify-between gap-3 border-b border-app-line bg-app-bg px-5 py-3 text-xs text-app-muted">
            <span>
              Showing {visibleMessages.length} of {messages.length} items{hiddenMessageCount > 0 && !showFullTrace ? ` · ${hiddenMessageCount} trace items hidden` : ""}
            </span>
            <label className="toggle">
              <input type="checkbox" checked={showFullTrace} onChange={(event) => setShowFullTrace(event.target.checked)} />
              Show full trace
            </label>
          </div>
        ) : null}

        <div className="min-h-0 flex-1 overflow-hidden">
          {loading ? (
            <div className="grid min-h-[360px] place-items-center text-sm text-app-muted">Loading conversation...</div>
          ) : error ? (
            <div className="px-5 py-5"><ErrorState message={error} /></div>
          ) : conversation && !conversation.supported ? (
            <div className="px-5 py-5"><EmptyState message={conversation.unavailableHint || "Conversation preview is not supported for this session."} /></div>
          ) : messages.length === 0 ? (
            <div className="px-5 py-5"><EmptyState message={conversation?.unavailableHint || "No conversation messages found."} /></div>
          ) : visibleMessages.length === 0 ? (
            <div className="px-5 py-5"><EmptyState message="Only hidden trace items found. Toggle full trace to view them." /></div>
          ) : (
            <AssistantUiTranscript messages={visibleMessages} />
          )}
        </div>
      </div>
    </div>
  );
}

function SessionDetailModal({
  session,
  noCost,
  onClose,
  onOpenSession,
  onOpenPath,
}: {
  session: IndexedSession;
  noCost: boolean;
  onClose: () => void;
  onOpenSession: (session: IndexedSession) => void;
  onOpenPath: (path: string) => void;
}) {
  const [tab, setTab] = useState<"overview" | "activity">("overview");
  const mcpRows = (session.activity?.operations ?? []).filter(isMcpActivityRow);

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") onClose();
    }
    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 px-6 py-6" onClick={onClose}>
      <div className="flex max-h-full min-h-0 w-full max-w-5xl flex-col overflow-hidden rounded-lg border border-app-line bg-app-bg shadow-2xl" onClick={(event) => event.stopPropagation()}>
        <div className="flex items-start justify-between gap-4 border-b border-app-line bg-app-panel px-5 py-4">
          <div className="min-w-0">
            <div className="text-lg font-semibold">Session details</div>
            <div className="mt-1 truncate text-xs text-app-muted">
              {session.agent}{session.model ? ` · ${session.model}` : ""}{session.reasoningLevel ? ` · ${session.reasoningLevel}` : ""} · {shortSessionId(session.sessionId)} · {formatDuration(session.activeDurationSeconds)} · {cleanProjectPath(session.projectPath)}
            </div>
          </div>
          <div className="flex items-center gap-2">
            {session.messageSourcePath ? (
              <button className="button" onClick={() => onOpenPath(session.messageSourcePath)}>
                <FolderOpen size={15} />
                Reveal transcript
              </button>
            ) : null}
            <button className="button" onClick={() => onOpenSession(session)}>
              <MessageSquare size={15} />
              Conversation
            </button>
            <button className="icon-button" onClick={onClose} title="Close">
              <X size={17} />
            </button>
          </div>
        </div>

        <div className="border-b border-app-line bg-app-bg px-5 py-3">
          <div className="segmented w-fit">
            {(["overview", "activity"] as const).map((key) => (
              <button key={key} className={tab === key ? "active" : ""} onClick={() => setTab(key)}>
                {key[0].toUpperCase() + key.slice(1)}
              </button>
            ))}
          </div>
        </div>

        <div className="min-h-0 overflow-y-auto px-5 py-4">
          {tab === "overview" ? (
            <div className="space-y-4">
              <div className="grid grid-cols-4 gap-3">
                <DetailStat label="Cost" value={formatCost(session.totalCost, noCost)} />
                <DetailStat label="Total tokens" value={formatTokens(session.totalTokens)} />
                <DetailStat label="Cache" value={formatTokens(session.cacheReadTokens + session.cacheCreationTokens)} />
                <DetailStat label="Duration" value={formatDuration(session.activeDurationSeconds)} />
              </div>
              <div>
                <h3 className="section-title">Last user message</h3>
                <div className="rounded-md border border-app-line bg-app-surface px-3 py-2 text-sm text-app-text">
                  {session.lastUserMessage || "No preview available."}
                </div>
              </div>
              {mcpRows.length > 0 ? <McpBreakdownChart rows={mcpRows} noCost={noCost} /> : null}
              <ModelBreakdownTable models={session.modelBreakdowns ?? []} noCost={noCost} />
            </div>
          ) : null}

          {tab === "activity" ? <ActivityUsagePanel activity={session.activity} noCost={noCost} /> : null}
        </div>
      </div>
    </div>
  );
}

function AssistantUiTranscript({messages}: {messages: ConversationMessage[]}) {
  const runtimeMessages = useMemo(() => conversationMessagesToAssistantUi(messages) as any[], [messages]);
  const runtime = useExternalStoreRuntime({
    messages: runtimeMessages,
    convertMessage: (message: any) => message,
    onNew: async () => undefined,
    isDisabled: true,
  });

  return (
    <AssistantRuntimeProvider runtime={runtime}>
      <Thread />
    </AssistantRuntimeProvider>
  );
}

function conversationMessagesToAssistantUi(messages: ConversationMessage[]) {
  const generatedIdsByOriginal = new Map<string, string>();
  return messages.map((message, index) => {
    const originalId = message.id || `${message.timestamp}-${message.role}-${message.type}`;
    const id = `${originalId}-${index}`;
    if (message.id && !generatedIdsByOriginal.has(message.id)) {
      generatedIdsByOriginal.set(message.id, id);
    }
    const createdAt = parseConversationDate(message.timestamp);
    const parentId = message.parentId ? (generatedIdsByOriginal.get(message.parentId) ?? null) : null;
    const common = {id, createdAt, parentId, metadata: {custom: {original: message}}};

    if (message.role === "user") {
      return {...common, role: "user", content: [{type: "text", text: message.text}], attachments: []};
    }

    if (message.role === "event") {
      return {...common, role: "system", content: [{type: "text", text: message.text}]};
    }

    if (message.type === "thinking") {
      return assistantUiAssistantMessage(common, [{type: "reasoning", text: message.text}]);
    }

    if (message.type === "toolCall") {
      const {title, json} = splitTracePayload(message.text);
      return assistantUiAssistantMessage(common, [{type: "tool-call", toolCallId: message.toolCallId || id, toolName: message.toolName || title || "tool", args: parseJSONOrText(json), argsText: json || ""}]);
    }

    if (message.role === "toolResult") {
      return assistantUiAssistantMessage(common, [{type: "tool-call", toolCallId: message.toolCallId || id, toolName: message.toolName || "tool", args: {}, argsText: "", result: message.text, isError: message.isError}]);
    }

    if (message.type === "image") {
      return assistantUiAssistantMessage(common, [{type: "text", text: "Image attachment"}]);
    }

    return assistantUiAssistantMessage(common, [{type: "text", text: message.text}]);
  });
}

function assistantUiAssistantMessage(common: Record<string, unknown>, content: unknown[]) {
  return {
    ...common,
    role: "assistant",
    content,
    status: {type: "complete", reason: "stop"},
  };
}

function splitTracePayload(text: string) {
  const [title, ...rest] = text.split("\n");
  return {title: title.trim(), json: rest.join("\n").trim()};
}

function parseJSONOrText(value: string) {
  if (!value) return {};
  try {
    return JSON.parse(value);
  } catch {
    return {text: value};
  }
}

function parseConversationDate(value: string) {
  const time = Date.parse(value);
  return Number.isNaN(time) ? new Date() : new Date(time);
}

function LegacyConversationModal({
  session,
  conversation,
  loading,
  error,
  onClose,
  onOpenSource,
}: {
  session: IndexedSession;
  conversation: SessionConversationResponse | null;
  loading: boolean;
  error: string;
  onClose: () => void;
  onOpenSource: (path: string) => void;
}) {
  const messages = conversation?.messages ?? [];
  const [showFullTrace, setShowFullTrace] = useState(false);
  const visibleMessages = showFullTrace ? messages : messages.filter((message) => !message.hiddenByDefault);
  const hiddenMessageCount = messages.length - visibleMessages.length;
  const messagesEndRef = useRef<HTMLDivElement | null>(null);

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        onClose();
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  useEffect(() => {
    if (!loading && visibleMessages.length > 0) {
      requestAnimationFrame(() => messagesEndRef.current?.scrollIntoView({block: "end"}));
    }
  }, [loading, visibleMessages.length]);

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 px-6 py-6" onClick={onClose}>
      <div className="flex max-h-full w-full max-w-5xl flex-col overflow-hidden rounded-lg border border-app-line bg-app-bg shadow-2xl" onClick={(event) => event.stopPropagation()}>
        <div className="flex items-start justify-between gap-4 border-b border-app-line bg-app-panel px-5 py-4">
          <div className="min-w-0">
            <div className="flex items-center gap-2 text-lg font-semibold">
              <MessageSquare size={18} />
              Conversation
            </div>
            <div className="mt-1 truncate text-xs text-app-muted">
              {session.agent}{conversationClientLabel(conversation, session) ? ` · ${conversationClientLabel(conversation, session)}` : ""}{conversationModelLabel(conversation, session) ? ` · ${conversationModelLabel(conversation, session)}` : ""} · {shortSessionId(session.sessionId)} · Duration {formatDuration(conversation?.activeDurationSeconds || session.activeDurationSeconds)} · {cleanProjectPath(session.projectPath)}
            </div>
          </div>
          <div className="flex items-center gap-2">
            {conversation?.sourcePath ? (
              <button className="button" onClick={() => onOpenSource(conversation.sourcePath)}>
                <FolderOpen size={15} />
                Reveal transcript
              </button>
            ) : null}
            <button className="icon-button" onClick={onClose} title="Close">
              <X size={17} />
            </button>
          </div>
        </div>

        {messages.length > 0 ? (
          <div className="flex items-center justify-between gap-3 border-b border-app-line bg-app-bg px-5 py-3 text-xs text-app-muted">
            <span>
              Showing {visibleMessages.length} of {messages.length} items{hiddenMessageCount > 0 && !showFullTrace ? ` · ${hiddenMessageCount} trace items hidden` : ""}
            </span>
            <label className="toggle">
              <input type="checkbox" checked={showFullTrace} onChange={(event) => setShowFullTrace(event.target.checked)} />
              Show full trace
            </label>
          </div>
        ) : null}

        <div className="min-h-[420px] flex-1 overflow-y-auto px-5 py-5">
          {loading ? (
            <div className="grid min-h-[360px] place-items-center text-sm text-app-muted">Loading conversation...</div>
          ) : error ? (
            <ErrorState message={error} />
          ) : conversation && !conversation.supported ? (
            <EmptyState message={conversation.unavailableHint || "Conversation preview is not supported for this session."} />
          ) : messages.length === 0 ? (
            <EmptyState message={conversation?.unavailableHint || "No conversation messages found."} />
          ) : visibleMessages.length === 0 ? (
            <EmptyState message="Only hidden trace items found. Toggle full trace to view them." />
          ) : (
            <div className="space-y-4">
              {visibleMessages.map((message, index) => (
                <ConversationBubble key={`${message.timestamp}-${message.role}-${message.type}-${index}`} message={message} />
              ))}
              <div ref={messagesEndRef} />
            </div>
          )}
        </div>
      </div>
    </div>
  );
}

function ConversationBubble({message}: {message: ConversationMessage}) {
  const isUser = message.role === "user";
  const isInfo = message.role === "event" && !message.hiddenByDefault;
  const isTrace = message.hiddenByDefault || message.role === "toolResult" || message.role === "event" || message.type === "toolCall" || message.type === "thinking";
  const label = conversationMessageLabel(message);
  if (isInfo) {
    return (
      <div className="flex justify-center text-xs italic text-app-muted">
        <span>{message.text}</span>
      </div>
    );
  }
  return (
    <article className={["flex", isUser ? "justify-end" : "justify-start", message.parentId ? "pl-6" : ""].join(" ")}>
      <div
        className={[
          "max-w-[82%] rounded-lg border px-4 py-3",
          isUser
            ? "border-app-accent/40 bg-app-accentSoft/70"
            : isTrace
              ? message.isError
                ? "border-red-500/35 bg-red-950/20"
                : "border-app-line bg-app-panel/60"
              : "border-app-line bg-app-surface",
        ].join(" ")}
      >
        <div className="mb-2 flex items-center justify-between gap-3 text-xs text-app-muted">
          <span className={["font-semibold tracking-normal text-app-text", isTrace ? "" : "uppercase"].join(" ")}>{label}</span>
          <span>{formatConversationDateTime(message.timestamp)}</span>
        </div>
        <div className={["markdown-body text-sm leading-6", isTrace ? "text-app-muted" : "text-app-text"].join(" ")}>
          <ReactMarkdown remarkPlugins={[remarkGfm]}>{formatConversationMessageText(message)}</ReactMarkdown>
        </div>
      </div>
    </article>
  );
}

function conversationMessageLabel(message: ConversationMessage) {
  if (message.role === "user") return "You";
  if (message.role === "assistant" && message.type === "thinking") return "Thinking";
  if (message.role === "assistant" && message.type === "toolCall") return `Tool call${message.toolName ? ` · ${message.toolName}` : ""}`;
  if (message.role === "toolResult") return `Tool result${message.toolName ? ` · ${message.toolName}` : ""}${message.isError ? " · error" : ""}`;
  if (message.role === "event") return conversationEventLabel(message.type);
  if (message.type === "image") return "Image";
  return "Assistant";
}

function conversationEventLabel(type: string) {
  switch (type) {
    case "session":
      return "Session";
    case "model_change":
      return "model";
    case "thinking_level_change":
      return "thinking level";
    default:
      return "Event";
  }
}

function formatConversationMessageText(message: ConversationMessage) {
  if ((message.type === "toolCall" || message.role === "toolResult") && message.text.includes("\n")) {
    const [title, ...rest] = message.text.split("\n");
    return `${title}\n\n\`\`\`\n${rest.join("\n")}\n\`\`\``;
  }
  return message.text;
}

const usageChartMetrics: {key: UsageChartMetric; label: string}[] = [
  {key: "totalCost", label: "Cost"},
  {key: "totalTokens", label: "Tokens"},
  {key: "inputTokens", label: "Input"},
  {key: "outputTokens", label: "Output"},
  {key: "cacheReadTokens", label: "Cache read"},
];

function UsageTrendChart({
  rows,
  report,
  metric,
  selectedIndex,
  noCost,
  onMetricChange,
  onSelect,
}: {
  rows: ReportRow[];
  report: ReportKey;
  metric: UsageChartMetric;
  selectedIndex: number;
  noCost: boolean;
  onMetricChange: (metric: UsageChartMetric) => void;
  onSelect: (index: number) => void;
}) {
  const data = rows.map((row, index) => ({
    ...row,
    index,
    label: usageChartLabel(row, report),
    value: row[metric],
  }));
  const selectedPeriod = data[Math.min(selectedIndex, Math.max(data.length - 1, 0))];

  return (
    <section className="rounded-md border border-app-line bg-app-surface px-3 py-3">
      <div className="mb-3 flex items-start justify-between gap-3">
        <div>
          <div className="text-xs font-semibold uppercase tracking-normal text-app-muted">{activeReport(report).label} trend</div>
          <div className="mt-1 text-sm text-app-text">
            {selectedPeriod ? `${selectedPeriod.label} · ${formatUsageChartValue(selectedPeriod.value, metric, noCost)}` : "No data"}
          </div>
        </div>
        <select className="control h-8 text-xs" value={metric} onChange={(event) => onMetricChange(event.target.value as UsageChartMetric)}>
          {usageChartMetrics.map((item) => (
            <option key={item.key} value={item.key}>
              {item.label}
            </option>
          ))}
        </select>
      </div>
      <div className="h-52">
        <ResponsiveContainer width="100%" height="100%">
          <BarChart
            data={data}
            margin={{top: 8, right: 8, bottom: 0, left: 0}}
            onClick={(state) => {
              if (typeof state?.activeTooltipIndex === "number") {
                onSelect(state.activeTooltipIndex);
              }
            }}
          >
            <CartesianGrid stroke="rgb(var(--color-line))" strokeOpacity={0.35} vertical={false} />
            <XAxis dataKey="label" axisLine={false} tickLine={false} tick={{fill: "rgb(var(--color-muted))", fontSize: 11}} interval="preserveStartEnd" />
            <YAxis
              axisLine={false}
              tickLine={false}
              tick={{fill: "rgb(var(--color-muted))", fontSize: 11}}
              tickFormatter={(value) => compactChartValue(Number(value), metric, noCost)}
              width={52}
            />
            <Tooltip
              cursor={{fill: "rgb(var(--color-accent-soft))", opacity: 0.25}}
              content={({active, payload, label}) => {
                if (!active || !payload?.length) {
                  return null;
                }
                const value = Number(payload[0].value ?? 0);
                return (
                  <div className="rounded-md border border-app-line bg-app-bg px-3 py-2 text-xs shadow-xl">
                    <div className="font-medium text-app-text">{label}</div>
                    <div className="mt-1 text-app-muted">{formatUsageChartValue(value, metric, noCost)}</div>
                  </div>
                );
              }}
            />
            <Bar dataKey="value" radius={[4, 4, 0, 0]}>
              {data.map((item) => (
                <Cell key={item.period} fill={item.index === selectedIndex ? "rgb(var(--color-accent))" : "rgb(var(--color-accent-soft))"} />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      </div>
    </section>
  );
}

const projectChartMetrics: {key: ProjectChartMetric; label: string}[] = [
  {key: "totalCost", label: "Cost"},
  {key: "totalTokens", label: "Tokens"},
  {key: "sessionCount", label: "Sessions"},
  {key: "cacheReadTokens", label: "Cache read"},
];

type ProjectChartRow = IndexedAggregate & Partial<Pick<ProjectSummary, "projectName" | "projectPath">> & Partial<Pick<IndexGroup, "name">>;

function ProjectUsageChart({
  rows,
  groupBy,
  metric,
  selectedIndex,
  noCost,
  onMetricChange,
  onSelect,
}: {
  rows: ProjectChartRow[];
  groupBy: IndexGroupBy;
  metric: ProjectChartMetric;
  selectedIndex: number;
  noCost: boolean;
  onMetricChange: (metric: ProjectChartMetric) => void;
  onSelect: (index: number) => void;
}) {
  const data = rows.slice(0, 10).map((row, index) => ({
    ...row,
    index,
    label: projectChartLabel(row, groupBy),
    value: row[metric],
  }));
  const selectedItem = rows[Math.min(selectedIndex, Math.max(rows.length - 1, 0))];

  return (
    <section className="rounded-md border border-app-line bg-app-surface px-3 py-3">
      <div className="mb-3 flex items-start justify-between gap-3">
        <div>
          <div className="text-xs font-semibold uppercase tracking-normal text-app-muted">
            Top {groupBy === "project" ? "projects" : `${groupBy}s`}
          </div>
          <div className="mt-1 text-sm text-app-text">
            {selectedItem ? `${projectChartLabel(selectedItem, groupBy)} · ${formatProjectChartValue(selectedItem[metric], metric, noCost)}` : "No data"}
          </div>
        </div>
        <select className="control h-8 text-xs" value={metric} onChange={(event) => onMetricChange(event.target.value as ProjectChartMetric)}>
          {projectChartMetrics.map((item) => (
            <option key={item.key} value={item.key}>
              {item.label}
            </option>
          ))}
        </select>
      </div>
      <div style={{height: Math.max(96, data.length * 34 + 34)}}>
        <ResponsiveContainer width="100%" height="100%">
          <BarChart
            data={data}
            layout="vertical"
            margin={{top: 4, right: 8, bottom: 0, left: 8}}
            onClick={(state) => {
              if (typeof state?.activeTooltipIndex === "number") {
                onSelect(data[state.activeTooltipIndex]?.index ?? state.activeTooltipIndex);
              }
            }}
          >
            <CartesianGrid stroke="rgb(var(--color-line))" strokeOpacity={0.35} horizontal={false} />
            <XAxis
              type="number"
              axisLine={false}
              tickLine={false}
              tick={{fill: "rgb(var(--color-muted))", fontSize: 11}}
              tickFormatter={(value) => compactProjectChartValue(Number(value), metric, noCost)}
            />
            <YAxis
              type="category"
              dataKey="label"
              axisLine={false}
              tickLine={false}
              tick={{fill: "rgb(var(--color-muted))", fontSize: 11}}
              width={130}
            />
            <Tooltip
              cursor={{fill: "rgb(var(--color-accent-soft))", opacity: 0.25}}
              content={({active, payload, label}) => {
                if (!active || !payload?.length) {
                  return null;
                }
                const value = Number(payload[0].value ?? 0);
                return (
                  <div className="rounded-md border border-app-line bg-app-bg px-3 py-2 text-xs shadow-xl">
                    <div className="font-medium text-app-text">{label}</div>
                    <div className="mt-1 text-app-muted">{formatProjectChartValue(value, metric, noCost)}</div>
                  </div>
                );
              }}
            />
            <Bar dataKey="value" radius={[0, 4, 4, 0]}>
              {data.map((item) => (
                <Cell key={`${item.label}-${item.index}`} fill={item.index === selectedIndex ? "rgb(var(--color-accent))" : "rgb(var(--color-accent-soft))"} />
              ))}
            </Bar>
          </BarChart>
        </ResponsiveContainer>
      </div>
    </section>
  );
}

function Metric({icon: Icon, label, value}: {icon: typeof Activity; label: string; value: string}) {
  return (
    <div className="rounded-md border border-app-line bg-app-surface px-3 py-2">
      <div className="flex items-center gap-2 text-xs text-app-muted">
        <Icon size={14} />
        {label}
      </div>
      <div className="mt-1 truncate text-lg font-semibold">{value}</div>
    </div>
  );
}

function DetailStat({label, value}: {label: string; value: string}) {
  return (
    <div className="rounded-md border border-app-line bg-app-surface px-3 py-2">
      <div className="text-xs text-app-muted">{label}</div>
      <div className="mt-0.5 truncate text-base font-semibold">{value}</div>
    </div>
  );
}

function ProjectDetail({
  project,
  noCost,
  database,
  onOpenPath,
  onOpenSession,
  onOpenSessionDetail,
  previewCache,
  onPreviewLoaded,
}: {
  project: ProjectSummary;
  noCost: boolean;
  database: string;
  onOpenPath: (path: string) => void;
  onOpenSession: (session: IndexedSession) => void;
  onOpenSessionDetail: (session: IndexedSession) => void;
  previewCache: Record<string, SessionPreviewResponse>;
  onPreviewLoaded: (session: IndexedSession, preview: SessionPreviewResponse) => void;
}) {
  const [showPhysicalPaths, setShowPhysicalPaths] = useState(false);
  const physicalPathCount = project.physicalPaths?.length ?? 0;
  const hasGroupedPaths = physicalPathCount > 1;

  return (
    <>
      <div className="mb-4 rounded-md border border-app-line bg-app-surface px-3 py-3">
        <div className="flex flex-wrap items-center gap-x-4 gap-y-2 text-sm">
          <span className="font-semibold">{formatCost(project.totalCost, noCost)}</span>
          <span className="text-app-muted">{project.sessionCount} {project.sessionCount === 1 ? "session" : "sessions"}</span>
          <span className="text-app-muted">{formatTokens(project.totalTokens)} tokens</span>
          <span className="text-app-muted">{formatTokens(project.cacheReadTokens)} cache read</span>
          <div className="flex flex-wrap gap-1.5">
            <AgentNameChips agents={project.agents ?? ["unknown"]} />
          </div>
        </div>

        <div className="mt-2 flex min-w-0 items-center gap-2">
          <span className="shrink-0 text-xs font-semibold uppercase tracking-wide text-app-muted">Path</span>
          <code className="min-w-0 flex-1 truncate rounded border border-app-line bg-app-bg px-2 py-1 text-xs text-app-muted">
            {cleanProjectPath(project.projectPath)}
          </code>
          {project.pathExists ? (
            <button className="icon-button h-7 w-7 shrink-0" title="Reveal in Finder" onClick={() => onOpenPath(project.projectPath)}>
              <FolderOpen size={13} />
            </button>
          ) : null}
          {hasGroupedPaths ? (
            <button className="button h-7 shrink-0 px-2 text-xs" onClick={() => setShowPhysicalPaths(true)}>
              {physicalPathCount} paths
            </button>
          ) : null}
        </div>

        {hasGroupedPaths ? (
          <div className="mt-1 text-xs text-app-muted">Grouped by {project.groupingRule || "grouping rule"}</div>
        ) : !project.pathExists ? (
          <div className="mt-1 text-xs text-app-muted">This folder no longer exists on disk.</div>
        ) : null}
      </div>

      {showPhysicalPaths ? (
        <PhysicalPathsModal project={project} onClose={() => setShowPhysicalPaths(false)} onOpenPath={onOpenPath} />
      ) : null}

      {(project.activity?.operations ?? []).filter(isMcpActivityRow).length > 0 ? (
        <McpBreakdownChart rows={(project.activity?.operations ?? []).filter(isMcpActivityRow)} noCost={noCost} />
      ) : null}

      <RecentSessionsTable
        sessions={project.recentSessions ?? []}
        noCost={noCost}
        previewCache={previewCache}
        onPreviewLoaded={onPreviewLoaded}
        onOpenSession={onOpenSession}
        onOpenSessionDetail={onOpenSessionDetail}
        onOpenPath={onOpenPath}
      />

      <ModelBreakdownTable models={project.modelBreakdowns ?? []} noCost={noCost} />
    </>
  );
}

function ActivityUsagePanel({activity, noCost}: {activity?: ActivitySummary; noCost: boolean}) {
  const [view, setView] = useState<"surfaces" | "tools" | "integrations" | "operations">("surfaces");
  const rows = (activity?.[view] ?? []).slice(0, 8);
  const mcpRows = (activity?.operations ?? []).filter(isMcpActivityRow).slice(0, 8);
  const totals = activity?.totals;
  if (!totals || totals.calls === 0) return null;

  return (
    <div className="mt-4 rounded-md border border-app-line bg-app-surface p-3">
      <div className="mb-3 flex items-center justify-between gap-3">
        <div className="text-xs text-app-muted">
          {totals.calls} calls · ~{formatTokens(totals.estimatedTokens)} tokens · ~{formatCost(totals.estimatedCost, noCost)} cost share
        </div>
        <div className="segmented w-auto">
          {(["surfaces", "tools", "integrations", "operations"] as const).map((key) => (
            <button key={key} className={view === key ? "active" : ""} onClick={() => setView(key)}>
              {key === "surfaces" ? "Surface" : key === "tools" ? "Tools" : key === "integrations" ? "Integrations" : "Ops"}
            </button>
          ))}
        </div>
      </div>
      <ActivityBreakdownTable rows={rows} view={view} noCost={noCost} />

      {mcpRows.length > 0 ? <McpBreakdownChart rows={mcpRows} noCost={noCost} /> : null}
    </div>
  );
}

const mcpSegmentColors = [
  "rgb(var(--color-accent))",
  "#f59e0b",
  "#22c55e",
  "#38bdf8",
  "#a78bfa",
];

function McpBreakdownChart({rows, noCost}: {rows: ActivityBreakdown[]; noCost: boolean}) {
  const data = buildMcpStackedChartData(rows);

  return (
    <section className="mt-3 rounded-md border border-app-line bg-app-bg px-3 py-3">
      <div className="mb-2 flex items-start justify-between gap-3">
        <div>
          <div className="text-xs font-semibold uppercase tracking-wide text-app-muted">MCP breakdown</div>
          <div className="mt-1 text-xs text-app-muted">Top MCP servers, split by operation</div>
        </div>
      </div>
      <div style={{height: Math.max(90, data.length * 30 + 30)}}>
        <ResponsiveContainer width="100%" height="100%">
          <BarChart data={data} layout="vertical" margin={{top: 2, right: 8, bottom: 0, left: 8}}>
            <CartesianGrid stroke="rgb(var(--color-line))" strokeOpacity={0.3} horizontal={false} />
            <XAxis
              type="number"
              axisLine={false}
              tickLine={false}
              tick={{fill: "rgb(var(--color-muted))", fontSize: 10}}
              tickFormatter={(value) => formatTokens(Number(value))}
            />
            <YAxis
              type="category"
              dataKey="label"
              axisLine={false}
              tickLine={false}
              tick={{fill: "rgb(var(--color-muted))", fontSize: 10}}
              width={210}
            />
            <Tooltip
              cursor={{fill: "rgb(var(--color-accent-soft))", opacity: 0.2}}
              content={({active, payload, label}) => {
                if (!active || !payload?.length) return null;
                const row = payload[0].payload as McpStackedChartRow;
                const details = Object.entries(row.details).filter(([, detail]) => detail.estimatedTokens > 0 || detail.calls > 0);
                return (
                  <div className="max-w-sm rounded-md border border-app-line bg-app-bg px-3 py-2 text-xs shadow-xl">
                    <div className="font-medium text-app-text">{label}</div>
                    <div className="mt-2 space-y-1">
                      {details.map(([key, detail]) => (
                        <div key={key} className="flex items-start gap-2">
                          <span className="mt-1 h-2 w-2 shrink-0 rounded-sm" style={{background: mcpSegmentColors[Number(key.replace("segment", ""))]}} />
                          <div className="min-w-0">
                            <div className="truncate text-app-text">{detail.label}</div>
                            <div className="text-app-muted">{detail.calls} calls · ~{formatTokens(detail.estimatedTokens)} · ~{formatCost(detail.estimatedCost, noCost)}{detail.errors ? ` · ${detail.errors} errors` : ""}</div>
                          </div>
                        </div>
                      ))}
                    </div>
                  </div>
                );
              }}
            />
            {mcpSegmentColors.map((color, index) => (
              <Bar key={index} dataKey={`segment${index}`} stackId="mcp" radius={index === mcpSegmentColors.length - 1 ? [0, 4, 4, 0] : [0, 0, 0, 0]} fill={color} />
            ))}
          </BarChart>
        </ResponsiveContainer>
      </div>
      <div className="mt-2 flex flex-wrap gap-x-3 gap-y-1 text-[11px] text-app-muted">
        {mcpSegmentColors.map((color, index) => (
          <span key={color} className="inline-flex items-center gap-1">
            <span className="h-2 w-2 rounded-sm" style={{background: color}} />
            {index < 4 ? `Top ${index + 1}` : "Other"}
          </span>
        ))}
      </div>
    </section>
  );
}

type McpStackedChartRow = {
  label: string;
  segment0: number;
  segment1: number;
  segment2: number;
  segment3: number;
  segment4: number;
  details: Record<string, ActivityBreakdown & {label: string}>;
};

function buildMcpStackedChartData(rows: ActivityBreakdown[]): McpStackedChartRow[] {
  const grouped = new Map<string, ActivityBreakdown[]>();
  for (const row of rows) {
    const provider = row.provider || row.toolName || row.operation || row.surface || "unknown";
    grouped.set(provider, [...(grouped.get(provider) ?? []), row]);
  }

  return [...grouped.entries()]
    .map(([provider, providerRows]) => {
      const operations = providerRows
        .map((row) => ({...row, label: row.operation || row.toolName || "unknown"}))
        .sort((a, b) => {
          if (a.estimatedTokens === b.estimatedTokens) return b.calls - a.calls;
          return b.estimatedTokens - a.estimatedTokens;
        });
      const top = operations.slice(0, 4);
      const other = operations.slice(4).reduce<ActivityBreakdown & {label: string}>(
        (acc, row) => ({
          ...acc,
          calls: acc.calls + row.calls,
          errors: acc.errors + row.errors,
          inputChars: acc.inputChars + row.inputChars,
          outputChars: acc.outputChars + row.outputChars,
          estimatedTokens: acc.estimatedTokens + row.estimatedTokens,
          observedTokens: acc.observedTokens + row.observedTokens,
          estimatedCost: acc.estimatedCost + row.estimatedCost,
        }),
        {label: "Other", calls: 0, errors: 0, inputChars: 0, outputChars: 0, estimatedTokens: 0, observedTokens: 0, estimatedCost: 0},
      );
      const segments = [...top, other];
      return {
        label: provider,
        segment0: segments[0]?.estimatedTokens ?? 0,
        segment1: segments[1]?.estimatedTokens ?? 0,
        segment2: segments[2]?.estimatedTokens ?? 0,
        segment3: segments[3]?.estimatedTokens ?? 0,
        segment4: segments[4]?.estimatedTokens ?? 0,
        details: Object.fromEntries(segments.map((segment, index) => [`segment${index}`, segment])),
      };
    })
    .sort((a, b) => {
      const aTotal = a.segment0 + a.segment1 + a.segment2 + a.segment3 + a.segment4;
      const bTotal = b.segment0 + b.segment1 + b.segment2 + b.segment3 + b.segment4;
      return bTotal - aTotal;
    })
    .slice(0, 8);
}

function ActivityBreakdownTable({
  rows,
  view,
  noCost,
}: {
  rows: ActivityBreakdown[];
  view: "surfaces" | "tools" | "integrations" | "operations";
  noCost: boolean;
}) {
  return (
    <div className="overflow-hidden rounded-md border border-app-line bg-app-bg">
      <table className="w-full text-sm">
        <thead className="border-b border-app-line bg-app-panel text-xs text-app-muted">
          <tr>
            <th className="table-head">Name</th>
            <th className="table-head text-right">Calls</th>
            <th className="table-head text-right">Est tokens</th>
            <th className="table-head text-right">Est cost</th>
            <th className="table-head text-right">Errors</th>
          </tr>
        </thead>
        <tbody>
          {rows.map((row, index) => (
            <tr key={`${activityRowLabel(row, view)}-${index}`} className="border-b border-app-line/70 last:border-0">
              <td className="table-cell font-medium">
                <div className="truncate">{activityRowLabel(row, view)}</div>
                {view === "operations" && row.provider ? <div className="mt-0.5 text-xs text-app-muted">{row.provider}</div> : null}
              </td>
              <td className="table-cell text-right">{row.calls}</td>
              <td className="table-cell text-right">~{formatTokens(row.estimatedTokens)}</td>
              <td className="table-cell text-right">~{formatCost(row.estimatedCost, noCost)}</td>
              <td className="table-cell text-right">{row.errors || "—"}</td>
            </tr>
          ))}
          {rows.length === 0 ? (
            <tr>
              <td className="table-cell text-app-muted" colSpan={5}>No activity found.</td>
            </tr>
          ) : null}
        </tbody>
      </table>
    </div>
  );
}

function isMcpActivityRow(row: ActivityBreakdown) {
  return row.surface === "mcp" || row.toolName === "mcp";
}

function activityRowLabel(row: ActivityBreakdown, view: "surfaces" | "tools" | "integrations" | "operations") {
  if (view === "surfaces") return row.surface || "unknown";
  if (view === "tools") return row.toolName || row.surface || "tool";
  if (view === "integrations") return row.provider || row.toolName || "integration";
  return row.operation || row.toolName || row.provider || "operation";
}

function ProjectOverviewSection({
  rows,
  groupBy,
  metric,
  selectedIndex,
  totals,
  noCost,
  onMetricChange,
  onSelect,
}: {
  rows: ProjectSummary[];
  groupBy: IndexGroupBy;
  metric: ProjectChartMetric;
  selectedIndex: number;
  totals: Pick<IndexedAggregate, "totalCost" | "totalTokens" | "inputTokens" | "outputTokens">;
  noCost: boolean;
  onMetricChange: (metric: ProjectChartMetric) => void;
  onSelect: (index: number) => void;
}) {
  if (rows.length === 0) return null;

  return (
    <div className="mt-6 border-t border-app-line pt-5">
      <h3 className="section-title">Project analytics</h3>
      <ProjectUsageChart
        rows={rows}
        groupBy={groupBy}
        metric={metric}
        selectedIndex={selectedIndex}
        noCost={noCost}
        onMetricChange={onMetricChange}
        onSelect={onSelect}
      />
      <div className="mt-4 grid grid-cols-4 gap-3">
        <Metric icon={DollarSign} label="Cost" value={formatCost(totals.totalCost, noCost)} />
        <Metric icon={Command} label="Tokens" value={formatTokens(totals.totalTokens)} />
        <Metric icon={Clock3} label="Input" value={formatTokens(totals.inputTokens)} />
        <Metric icon={Activity} label="Output" value={formatTokens(totals.outputTokens)} />
      </div>
    </div>
  );
}

function PhysicalPathsModal({
  project,
  onClose,
  onOpenPath,
}: {
  project: ProjectSummary;
  onClose: () => void;
  onOpenPath: (path: string) => void;
}) {
  const paths = project.physicalPaths ?? [];

  useEffect(() => {
    function handleKeyDown(event: KeyboardEvent) {
      if (event.key === "Escape") {
        onClose();
      }
    }

    window.addEventListener("keydown", handleKeyDown);
    return () => window.removeEventListener("keydown", handleKeyDown);
  }, [onClose]);

  return (
    <div className="fixed inset-0 z-50 grid place-items-center bg-black/60 px-6 py-6" onClick={onClose}>
      <div className="flex max-h-full w-full max-w-4xl flex-col overflow-hidden rounded-lg border border-app-line bg-app-bg shadow-2xl" onClick={(event) => event.stopPropagation()}>
        <div className="flex items-start justify-between gap-4 border-b border-app-line bg-app-panel px-5 py-4">
          <div className="min-w-0">
            <div className="text-lg font-semibold">Physical paths</div>
            <div className="mt-1 text-xs text-app-muted">
              {project.projectName} · grouped by {project.groupingRule || "grouping rule"}
            </div>
          </div>
          <button className="icon-button" onClick={onClose} title="Close">
            <X size={17} />
          </button>
        </div>

        <div className="overflow-y-auto px-5 py-5">
          <div className="mb-4">
            <h3 className="section-title">Path</h3>
            <code className="block overflow-x-auto rounded-md border border-app-line bg-app-surface px-3 py-2 text-xs text-app-muted">
              {cleanProjectPath(project.projectPath)}
            </code>
          </div>

          <h3 className="section-title">Physical Paths · {paths.length}</h3>
          <div className="space-y-2">
            {paths.map((path) => (
              <div key={path} className="flex items-center gap-2 rounded-md border border-app-line bg-app-surface px-3 py-2">
                <code className="min-w-0 flex-1 overflow-x-auto text-xs text-app-muted">{cleanProjectPath(path)}</code>
                <button className="icon-button h-7 w-7 shrink-0" title="Reveal in Finder" onClick={() => onOpenPath(path)}>
                  <FolderOpen size={13} />
                </button>
              </div>
            ))}
          </div>
        </div>
      </div>
    </div>
  );
}

function IndexGroupDetail({group, noCost, database}: {group: IndexGroup; noCost: boolean; database: string}) {
  return (
    <>
      <div className="grid grid-cols-4 gap-3">
        <DetailStat label="Cost" value={formatCost(group.totalCost, noCost)} />
        <DetailStat label="Sessions" value={String(group.sessionCount)} />
        <DetailStat label="Projects" value={String(group.projectCount)} />
        <DetailStat label="Total tokens" value={formatTokens(group.totalTokens)} />
      </div>

      <div className="mt-6">
        <h3 className="section-title">Agents</h3>
        <div className="flex flex-wrap gap-2">
          <AgentNameChips agents={group.agents ?? ["unknown"]} />
        </div>
      </div>

      <ModelBreakdownTable models={group.modelBreakdowns ?? []} noCost={noCost} />
    </>
  );
}

function ModelBreakdownTable({models, noCost}: {models: ModelBreakdown[]; noCost: boolean}) {
  return (
    <div className="mt-6">
      <h3 className="section-title">Model Breakdown</h3>
      <div className="overflow-hidden rounded-md border border-app-line bg-app-surface">
        <table className="w-full text-sm">
          <thead className="border-b border-app-line bg-app-panel text-xs text-app-muted">
            <tr>
              <th className="table-head">Model</th>
              <th className="table-head text-right">Cost</th>
              <th className="table-head text-right">Input</th>
              <th className="table-head text-right">Output</th>
              <th className="table-head text-right">Cache read</th>
            </tr>
          </thead>
          <tbody>
            {models.map((model) => (
              <tr key={model.modelName} className="border-b border-app-line/70 last:border-0">
                <td className="table-cell font-medium">{model.modelName}</td>
                <td className="table-cell text-right">{formatCost(model.cost, noCost)}</td>
                <td className="table-cell text-right">{formatTokens(model.inputTokens)}</td>
                <td className="table-cell text-right">{formatTokens(model.outputTokens)}</td>
                <td className="table-cell text-right">{formatTokens(model.cacheReadTokens)}</td>
              </tr>
            ))}
            {models.length === 0 ? (
              <tr>
                <td className="table-cell text-app-muted" colSpan={5}>
                  No model breakdown available.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function RecentSessionsTable({
  sessions,
  noCost,
  previewCache,
  onPreviewLoaded,
  onOpenSession,
  onOpenSessionDetail,
  onOpenPath,
}: {
  sessions: IndexedSession[];
  noCost: boolean;
  previewCache: Record<string, SessionPreviewResponse>;
  onPreviewLoaded: (session: IndexedSession, preview: SessionPreviewResponse) => void;
  onOpenSession: (session: IndexedSession) => void;
  onOpenSessionDetail: (session: IndexedSession) => void;
  onOpenPath: (path: string) => void;
}) {
  const [sessionSort, setSessionSort] = useState<SortField>("lastActivity");
  const [pageSize, setPageSize] = useState(10);
  const [page, setPage] = useState(0);

  const sortedSessions = useMemo(() => sortUsageItems(sessions, sessionSort), [sessionSort, sessions]);
  const pageCount = Math.max(1, Math.ceil(sortedSessions.length / pageSize));
  const safePage = Math.min(page, pageCount - 1);
  const pagedSessions = sortedSessions.slice(safePage * pageSize, safePage * pageSize + pageSize);

  useEffect(() => {
    setPage(0);
  }, [sessions, sessionSort, pageSize]);

  return (
    <div className="mt-4">
      <div className="mb-2 flex items-center justify-between gap-3">
        <h3 className="section-title mb-0">Sessions</h3>
        <div className="flex items-center gap-2 text-xs text-app-muted">
          <span>Sort</span>
          <select className="control h-8" value={sessionSort} onChange={(event) => setSessionSort(event.target.value as SortField)}>
            <option value="lastActivity">Last activity</option>
            <option value="totalCost">Cost</option>
            <option value="totalTokens">Total tokens</option>
          </select>
          <select className="control h-8" value={pageSize} onChange={(event) => setPageSize(Number(event.target.value))}>
            {pageSizeOptions.map((size) => (
              <option key={size} value={size}>{size}/page</option>
            ))}
          </select>
        </div>
      </div>
      <div className="overflow-hidden rounded-md border border-app-line bg-app-surface">
        <table className="w-full text-sm">
          <thead className="border-b border-app-line bg-app-panel text-xs text-app-muted">
            <tr>
              <th className="table-head">Agent</th>
              <th className="table-head">Last User Message</th>
              <th className="table-head text-right">Cost</th>
              <th className="table-head text-right">Last activity</th>
            </tr>
          </thead>
          <tbody>
            {pagedSessions.map((session) => {
              const key = sessionPreviewKey(session);
              return (
              <Fragment key={`${session.agent}-${session.sessionId}`}>
              <tr className="cursor-pointer border-b border-app-line/70 transition hover:bg-app-panel/60 last:border-0" onClick={() => onOpenSessionDetail(session)}>
                <td className="table-cell">
                  <SessionAgentCell session={session} cachedPreview={previewCache[sessionPreviewKey(session)]} />
                </td>
                <td className="table-cell max-w-[360px] lg:max-w-[460px]" title={session.sessionId}>
                  <SessionPreviewText
                    session={session}
                    cachedPreview={previewCache[sessionPreviewKey(session)]}
                    onPreviewLoaded={(preview) => onPreviewLoaded(session, preview)}
                  />
                </td>
                <td className="table-cell whitespace-nowrap text-right">{formatCost(session.totalCost, noCost)}</td>
                <td className="table-cell whitespace-nowrap text-right text-app-muted">{formatDateTime(session.lastActivity)}</td>
              </tr>
              </Fragment>
            );
            })}
            {sortedSessions.length === 0 ? (
              <tr>
                <td className="table-cell text-app-muted" colSpan={4}>
                  No indexed sessions available.
                </td>
              </tr>
            ) : null}
          </tbody>
        </table>
      </div>
      {sortedSessions.length > pageSize ? (
        <div className="mt-3 flex items-center justify-between text-xs text-app-muted">
          <span>
            {safePage * pageSize + 1}–{Math.min((safePage + 1) * pageSize, sortedSessions.length)} of {sortedSessions.length}
          </span>
          <div className="flex gap-2">
            <button className="button h-8" disabled={safePage === 0} onClick={() => setPage((current) => Math.max(0, current - 1))}>
              Previous
            </button>
            <button className="button h-8" disabled={safePage >= pageCount - 1} onClick={() => setPage((current) => Math.min(pageCount - 1, current + 1))}>
              Next
            </button>
          </div>
        </div>
      ) : null}
    </div>
  );
}

function SessionAgentCell({session, cachedPreview}: {session: IndexedSession; cachedPreview?: SessionPreviewResponse}) {
  const originator = cachedPreview?.originator || session.originator;
  const clientSource = cachedPreview?.clientSource || session.clientSource;
  const model = cachedPreview?.model || session.model;
  const reasoningLevel = cachedPreview?.reasoningLevel || session.reasoningLevel;
  const client = [originator, clientSource].filter(Boolean).join(" · ");
  const modelInfo = [model, reasoningLevel].filter(Boolean).join(" · ");
  return (
    <div className="space-y-1">
      <AgentNameChips agents={[session.agent]} />
      {client ? <div className="text-[11px] leading-4 text-app-muted">{client}</div> : null}
      {modelInfo ? <div className="text-[11px] leading-4 text-app-muted">{modelInfo}</div> : null}
    </div>
  );
}

function SessionPreviewText({
  session,
  cachedPreview,
  onPreviewLoaded,
}: {
  session: IndexedSession;
  cachedPreview?: SessionPreviewResponse;
  onPreviewLoaded: (preview: SessionPreviewResponse) => void;
}) {
  const initialPreview = cachedPreview?.preview || session.lastUserMessage || cachedPreview?.unavailableHint || "";
  const [preview, setPreview] = useState(initialPreview);
  const [sourcePath, setSourcePath] = useState(cachedPreview?.sourcePath || session.messageSourcePath || "");
  const [loading, setLoading] = useState(!initialPreview);

  useEffect(() => {
    let cancelled = false;
    if (cachedPreview?.preview || cachedPreview?.unavailableHint) {
      setPreview(cachedPreview.preview || cachedPreview.unavailableHint || "No user message found");
      setSourcePath(cachedPreview.sourcePath || "");
      setLoading(false);
      return;
    }

    if (session.lastUserMessage) {
      setPreview(session.lastUserMessage);
      setSourcePath(session.messageSourcePath ?? "");
      setLoading(false);
      return;
    }

    setLoading(true);
    GetSessionPreview({
      agent: session.agent,
      sessionId: session.sessionId,
      projectPath: session.projectPath,
    })
      .then((response) => {
        if (cancelled) {
          return;
        }
        const typed = response as SessionPreviewResponse;
        setPreview(typed.preview || typed.unavailableHint || "No user message found");
        setSourcePath(typed.sourcePath || "");
        onPreviewLoaded(typed);
      })
      .catch((err) => {
        if (!cancelled) {
          setPreview(isDatabaseLockError(err) ? "Preview is busy, retrying on next refresh" : "Preview unavailable");
        }
      })
      .finally(() => {
        if (!cancelled) {
          setLoading(false);
        }
      });

    return () => {
      cancelled = true;
    };
  }, [cachedPreview, onPreviewLoaded, session.agent, session.lastUserMessage, session.messageSourcePath, session.projectPath, session.sessionId]);

  return (
    <span className="flex items-start gap-2">
      <span className="line-clamp-2 min-w-0 flex-1 text-xs text-app-muted">{loading ? "Loading..." : preview}</span>

    </span>
  );
}

function AgentChips({row}: {row: ReportRow}) {
  return <AgentNameChips agents={agentNames(row)} maxVisible={3} />;
}

function AgentNameChips({agents, maxVisible}: {agents: string[]; maxVisible?: number}) {
  const visibleAgents = maxVisible ? agents.slice(0, maxVisible) : agents;
  const hiddenCount = maxVisible ? Math.max(agents.length - maxVisible, 0) : 0;
  return (
    <span className={["flex min-w-0 items-center gap-1.5", maxVisible ? "" : "flex-wrap"].join(" ")}>
      {visibleAgents.map((agent) => (
        <span
          key={agent}
          className="max-w-[78px] truncate rounded bg-app-accentSoft px-1.5 py-0.5 text-[11px] font-medium text-app-text"
        >
          {agent}
        </span>
      ))}
      {hiddenCount > 0 ? <span className="text-[11px] text-app-muted">+{hiddenCount}</span> : null}
    </span>
  );
}

function ErrorState({message}: {message: string}) {
  return (
    <div className="m-4 rounded-md border border-red-200 bg-red-50 px-3 py-3 text-sm text-red-900">
      {message}
    </div>
  );
}

function EmptyState({
  message = "No usage found for this filter.",
  actionLabel,
  onAction,
}: {
  message?: string;
  actionLabel?: string;
  onAction?: () => void;
}) {
  return (
    <div className="px-4 py-10 text-center text-sm text-app-muted">
      <div>{message}</div>
      {actionLabel && onAction ? (
        <button className="button mx-auto mt-4" onClick={onAction}>
          <RefreshCw size={15} />
          {actionLabel}
        </button>
      ) : null}
    </div>
  );
}

function activeReport(report: ReportKey) {
  return reports.find((item) => item.key === report) ?? reports[0];
}

function rowTitle(row: ReportRow, report: ReportKey) {
  if (report === "daily") {
    const date = parseDateOnly(row.period);
    return date ? formatDateOnly(date) : row.period || "Unknown period";
  }
  if (report === "weekly") {
    return periodRangeLabel(row.period, 6);
  }
  if (report === "monthly") {
    return formatMonthPeriod(row.period);
  }
  if (report === "session") {
    const projectPath = row.metadata?.projectPath;
    if (typeof projectPath === "string" && projectPath.length > 0) {
      return cleanProjectPath(projectPath);
    }
  }
  return row.period || "Unknown period";
}

function periodRangeLabel(period: string, endOffsetDays: number) {
  const start = parseDateOnly(period);
  if (!start) {
    return period || "Unknown period";
  }
  const end = new Date(start);
  end.setUTCDate(end.getUTCDate() + endOffsetDays);
  return `${formatDateOnly(start)} – ${formatDateOnly(end)}`;
}

function parseDateOnly(value: string) {
  const match = /^(\d{4})-(\d{2})-(\d{2})$/.exec(value);
  if (!match) {
    return null;
  }
  return new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1, Number(match[3])));
}

function formatDateOnly(value: Date) {
  return new Intl.DateTimeFormat("en-GB", {
    day: "numeric",
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  }).format(value);
}

function formatMonthPeriod(period: string) {
  const match = /^(\d{4})-(\d{2})/.exec(period);
  if (!match) {
    return period || "Unknown period";
  }
  return new Intl.DateTimeFormat("en-GB", {
    month: "short",
    year: "numeric",
    timeZone: "UTC",
  }).format(new Date(Date.UTC(Number(match[1]), Number(match[2]) - 1, 1)));
}

function cleanProjectPath(value: string) {
  const decoded = decodeProjectPath(value).replace(/^\//, "");
  const homeMatch = /^Users\/[^/]+(?:\/(.*))?$/.exec(decoded);
  if (homeMatch) {
    return homeMatch[1] ? `~/${homeMatch[1]}` : "~";
  }
  return decoded;
}

function decodeProjectPath(value: string) {
  if (!value.startsWith("--")) {
    return value.replace(/^\//, "");
  }

  const parts = value.replace(/^--/, "").replace(/--$/, "").split("-").filter(Boolean);
  const workspaceIndex = parts.findIndex((part, index) => part === "workspace" && parts[index + 1] === "worktrees");
  const projectsIndex = parts.indexOf("projects", workspaceIndex + 4);
  if (workspaceIndex >= 0 && projectsIndex > workspaceIndex + 5) {
    const prefix = parts.slice(0, workspaceIndex + 4);
    const branchName = parts.slice(workspaceIndex + 4, projectsIndex).join("-");
    const suffix = parts.slice(projectsIndex);
    return [...prefix, branchName, ...suffix].join("/");
  }

  return parts.join("/");
}

function shortSessionId(value: string) {
  const segments = value.split(/[-/]/).filter(Boolean);
  return segments.at(-1) ?? value;
}

function sessionPreviewKey(session: Pick<IndexedSession, "agent" | "sessionId">) {
  return `${session.agent || "all"}:${session.sessionId}`;
}

function conversationClientLabel(conversation: SessionConversationResponse | null, session: IndexedSession) {
  return [conversation?.originator || session.originator, conversation?.clientSource || session.clientSource]
    .filter(Boolean)
    .join(" · ");
}

function conversationModelLabel(conversation: SessionConversationResponse | null, session: IndexedSession) {
  return [conversation?.model || session.model, conversation?.reasoningLevel || session.reasoningLevel]
    .filter(Boolean)
    .join(" · ");
}

function modelLabel(row: ReportRow) {
  const models = row.modelsUsed ?? [];
  if (models.length === 0) {
    return "no models";
  }
  if (models.length === 1) {
    return models[0];
  }
  return `${models.length} models`;
}

function agentNames(row: ReportRow) {
  const metadataAgents = row.metadata?.agents;
  if (Array.isArray(metadataAgents)) {
    const agents = metadataAgents.filter((agent): agent is string => typeof agent === "string" && agent.length > 0);
    if (agents.length > 0) {
      return agents;
    }
  }

  return [row.agent || "all"];
}

function sortUsageItems<T extends {lastActivity: string; totalCost: number; totalTokens: number}>(items: T[], field: SortField) {
  return [...items].sort((left, right) => {
    if (field === "lastActivity") {
      return dateValue(right.lastActivity) - dateValue(left.lastActivity);
    }
    return (right[field] || 0) - (left[field] || 0);
  });
}

function dateValue(value: string) {
  const timestamp = new Date(value).getTime();
  return Number.isNaN(timestamp) ? 0 : timestamp;
}

function ccusageUntilDate(value: string) {
  return addDaysToDateOnly(value, 1);
}

function addDaysToDateOnly(value: string, days: number) {
  const date = parseDateOnly(value);
  if (!date) {
    return value;
  }
  date.setUTCDate(date.getUTCDate() + days);
  return date.toISOString().slice(0, 10);
}

function formatCost(value: number, noCost: boolean) {
  if (noCost) {
    return "hidden";
  }
  return new Intl.NumberFormat("en-US", {style: "currency", currency: "USD", maximumFractionDigits: 2}).format(value || 0);
}

function formatTokens(value: number) {
  return new Intl.NumberFormat("en-US", {notation: "compact", maximumFractionDigits: 1}).format(value || 0);
}

function projectChartLabel(row: ProjectChartRow, groupBy: IndexGroupBy) {
  const label = groupBy === "project" ? row.projectName || cleanProjectPath(row.projectPath || "") : row.name;
  if (!label) {
    return "Unknown";
  }
  const runes = Array.from(label);
  return runes.length > 24 ? `${runes.slice(0, 23).join("")}…` : label;
}

function usageChartLabel(row: ReportRow, report: ReportKey) {
  if (report === "daily") {
    const date = parseDateOnly(row.period);
    return date
      ? new Intl.DateTimeFormat("en-GB", {day: "2-digit", month: "short"}).format(date)
      : row.period;
  }
  if (report === "weekly") {
    return row.period.replace(/^week-/, "W");
  }
  return formatMonthPeriod(row.period);
}

function formatProjectChartValue(value: number, metric: ProjectChartMetric, noCost: boolean) {
  if (metric === "totalCost") {
    return formatCost(value, noCost);
  }
  if (metric === "sessionCount") {
    return new Intl.NumberFormat("en-US").format(value || 0);
  }
  return formatTokens(value);
}

function compactProjectChartValue(value: number, metric: ProjectChartMetric, noCost: boolean) {
  if (metric === "sessionCount") {
    return new Intl.NumberFormat("en-US", {notation: "compact", maximumFractionDigits: 1}).format(value || 0);
  }
  if (metric === "totalCost") {
    return compactChartValue(value, "totalCost", noCost);
  }
  return formatTokens(value);
}

function formatUsageChartValue(value: number, metric: UsageChartMetric, noCost: boolean) {
  if (metric === "totalCost") {
    return formatCost(value, noCost);
  }
  return formatTokens(value);
}

function compactChartValue(value: number, metric: UsageChartMetric, noCost: boolean) {
  if (metric === "totalCost" && noCost) {
    return "hidden";
  }
  if (metric === "totalCost") {
    return new Intl.NumberFormat("en-US", {style: "currency", currency: "USD", notation: "compact", maximumFractionDigits: 1}).format(value || 0);
  }
  return formatTokens(value);
}

function formatDuration(seconds: number) {
  if (!seconds || seconds <= 0) {
    return "—";
  }
  const rounded = Math.round(seconds);
  const hours = Math.floor(rounded / 3600);
  const minutes = Math.floor((rounded % 3600) / 60);
  const remainingSeconds = rounded % 60;
  if (hours > 0) {
    return minutes > 0 ? `${hours}h ${minutes}m` : `${hours}h`;
  }
  if (minutes > 0) {
    return remainingSeconds > 0 ? `${minutes}m ${remainingSeconds}s` : `${minutes}m`;
  }
  return `${remainingSeconds}s`;
}

function formatConversationDateTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("en-GB", {
    day: "2-digit",
    month: "short",
    hour: "numeric",
    minute: "2-digit",
    second: "2-digit",
    hour12: true,
  }).format(date);
}

function formatDateTime(value: string) {
  const date = new Date(value);
  if (Number.isNaN(date.getTime())) {
    return value;
  }
  return new Intl.DateTimeFormat("en-GB", {
    day: "2-digit",
    month: "short",
    hour: "numeric",
    minute: "2-digit",
    hour12: true,
  }).format(date);
}

function toNumber(value: unknown) {
  return typeof value === "number" ? value : 0;
}

function errorMessage(err: unknown) {
  if (err instanceof Error) {
    return err.message;
  }
  return String(err);
}

function isDatabaseLockError(err: unknown) {
  return String(err).toLowerCase().includes("database is locked");
}

export default App;
