export namespace main {
	
	export class ProjectGroupingRule {
	    name: string;
	    matchPath: string;
	    groupAs: string;
	    displayAs?: string;
	    pattern?: string;
	    groupPath?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProjectGroupingRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.matchPath = source["matchPath"];
	        this.groupAs = source["groupAs"];
	        this.displayAs = source["displayAs"];
	        this.pattern = source["pattern"];
	        this.groupPath = source["groupPath"];
	    }
	}
	export class ProjectGroupingConfig {
	    enabled: boolean;
	    rules: ProjectGroupingRule[];
	
	    static createFrom(source: any = {}) {
	        return new ProjectGroupingConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.rules = this.convertValues(source["rules"], ProjectGroupingRule);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class AppConfig {
	    projectGrouping: ProjectGroupingConfig;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projectGrouping = this.convertValues(source["projectGrouping"], ProjectGroupingConfig);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ConversationMessage {
	    role: string;
	    timestamp: string;
	    text: string;
	
	    static createFrom(source: any = {}) {
	        return new ConversationMessage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.timestamp = source["timestamp"];
	        this.text = source["text"];
	    }
	}
	export class ModelBreakdown {
	    modelName: string;
	    inputTokens: number;
	    outputTokens: number;
	    cacheCreationTokens: number;
	    cacheReadTokens: number;
	    cost: number;
	
	    static createFrom(source: any = {}) {
	        return new ModelBreakdown(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.modelName = source["modelName"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.cacheCreationTokens = source["cacheCreationTokens"];
	        this.cacheReadTokens = source["cacheReadTokens"];
	        this.cost = source["cost"];
	    }
	}
	export class IndexGroup {
	    name: string;
	    groupBy: string;
	    projectCount: number;
	    sessionCount: number;
	    lastActivity: string;
	    inputTokens: number;
	    outputTokens: number;
	    cacheCreationTokens: number;
	    cacheReadTokens: number;
	    totalTokens: number;
	    totalCost: number;
	    agents: string[];
	    modelBreakdowns: ModelBreakdown[];
	
	    static createFrom(source: any = {}) {
	        return new IndexGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.groupBy = source["groupBy"];
	        this.projectCount = source["projectCount"];
	        this.sessionCount = source["sessionCount"];
	        this.lastActivity = source["lastActivity"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.cacheCreationTokens = source["cacheCreationTokens"];
	        this.cacheReadTokens = source["cacheReadTokens"];
	        this.totalTokens = source["totalTokens"];
	        this.totalCost = source["totalCost"];
	        this.agents = source["agents"];
	        this.modelBreakdowns = this.convertValues(source["modelBreakdowns"], ModelBreakdown);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class IndexRequest {
	    source: string;
	    since: string;
	    until: string;
	    offline: boolean;
	    noCost: boolean;
	
	    static createFrom(source: any = {}) {
	        return new IndexRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.since = source["since"];
	        this.until = source["until"];
	        this.offline = source["offline"];
	        this.noCost = source["noCost"];
	    }
	}
	export class IndexedSession {
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
	    modelBreakdowns: ModelBreakdown[];
	    lastUserMessage: string;
	    lastUserMessageAt: string;
	    messageSourcePath: string;
	    activeDurationSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new IndexedSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.agent = source["agent"];
	        this.projectPath = source["projectPath"];
	        this.projectName = source["projectName"];
	        this.lastActivity = source["lastActivity"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.cacheCreationTokens = source["cacheCreationTokens"];
	        this.cacheReadTokens = source["cacheReadTokens"];
	        this.totalTokens = source["totalTokens"];
	        this.totalCost = source["totalCost"];
	        this.modelBreakdowns = this.convertValues(source["modelBreakdowns"], ModelBreakdown);
	        this.lastUserMessage = source["lastUserMessage"];
	        this.lastUserMessageAt = source["lastUserMessageAt"];
	        this.messageSourcePath = source["messageSourcePath"];
	        this.activeDurationSeconds = source["activeDurationSeconds"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	
	export class RunnerInfo {
	    name: string;
	    path: string;
	    args: string[];
	    available: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new RunnerInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.path = source["path"];
	        this.args = source["args"];
	        this.available = source["available"];
	        this.message = source["message"];
	    }
	}
	export class ProjectSummary {
	    projectPath: string;
	    projectName: string;
	    physicalPaths: string[];
	    groupingRule: string;
	    agents: string[];
	    sessionCount: number;
	    lastActivity: string;
	    inputTokens: number;
	    outputTokens: number;
	    cacheCreationTokens: number;
	    cacheReadTokens: number;
	    totalTokens: number;
	    totalCost: number;
	    modelBreakdowns: ModelBreakdown[];
	    recentSessions: IndexedSession[];
	
	    static createFrom(source: any = {}) {
	        return new ProjectSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projectPath = source["projectPath"];
	        this.projectName = source["projectName"];
	        this.physicalPaths = source["physicalPaths"];
	        this.groupingRule = source["groupingRule"];
	        this.agents = source["agents"];
	        this.sessionCount = source["sessionCount"];
	        this.lastActivity = source["lastActivity"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.cacheCreationTokens = source["cacheCreationTokens"];
	        this.cacheReadTokens = source["cacheReadTokens"];
	        this.totalTokens = source["totalTokens"];
	        this.totalCost = source["totalCost"];
	        this.modelBreakdowns = this.convertValues(source["modelBreakdowns"], ModelBreakdown);
	        this.recentSessions = this.convertValues(source["recentSessions"], IndexedSession);
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ProjectIndexResponse {
	    projects: ProjectSummary[];
	    agentGroups: IndexGroup[];
	    modelGroups: IndexGroup[];
	    database: string;
	    lastIndexed: string;
	    runner: RunnerInfo;
	    command: string[];
	    generated: string;
	
	    static createFrom(source: any = {}) {
	        return new ProjectIndexResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projects = this.convertValues(source["projects"], ProjectSummary);
	        this.agentGroups = this.convertValues(source["agentGroups"], IndexGroup);
	        this.modelGroups = this.convertValues(source["modelGroups"], IndexGroup);
	        this.database = source["database"];
	        this.lastIndexed = source["lastIndexed"];
	        this.runner = this.convertValues(source["runner"], RunnerInfo);
	        this.command = source["command"];
	        this.generated = source["generated"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	export class ReportRequest {
	    report: string;
	    source: string;
	    since: string;
	    until: string;
	    offline: boolean;
	    noCost: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ReportRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.report = source["report"];
	        this.source = source["source"];
	        this.since = source["since"];
	        this.until = source["until"];
	        this.offline = source["offline"];
	        this.noCost = source["noCost"];
	    }
	}
	export class ReportRow {
	    period: string;
	    agent: string;
	    inputTokens: number;
	    outputTokens: number;
	    cacheCreationTokens: number;
	    cacheReadTokens: number;
	    totalTokens: number;
	    totalCost: number;
	    modelsUsed: string[];
	    modelBreakdowns: ModelBreakdown[];
	    metadata: Record<string, any>;
	    raw: Record<string, any>;
	
	    static createFrom(source: any = {}) {
	        return new ReportRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.period = source["period"];
	        this.agent = source["agent"];
	        this.inputTokens = source["inputTokens"];
	        this.outputTokens = source["outputTokens"];
	        this.cacheCreationTokens = source["cacheCreationTokens"];
	        this.cacheReadTokens = source["cacheReadTokens"];
	        this.totalTokens = source["totalTokens"];
	        this.totalCost = source["totalCost"];
	        this.modelsUsed = source["modelsUsed"];
	        this.modelBreakdowns = this.convertValues(source["modelBreakdowns"], ModelBreakdown);
	        this.metadata = source["metadata"];
	        this.raw = source["raw"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class ReportResponse {
	    report: string;
	    source: string;
	    runner: RunnerInfo;
	    command: string[];
	    rows: ReportRow[];
	    totals: Record<string, any>;
	    generated: string;
	
	    static createFrom(source: any = {}) {
	        return new ReportResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.report = source["report"];
	        this.source = source["source"];
	        this.runner = this.convertValues(source["runner"], RunnerInfo);
	        this.command = source["command"];
	        this.rows = this.convertValues(source["rows"], ReportRow);
	        this.totals = source["totals"];
	        this.generated = source["generated"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	
	
	export class SessionConversationResponse {
	    sessionId: string;
	    agent: string;
	    projectPath: string;
	    sourcePath: string;
	    activeDurationSeconds: number;
	    messages: ConversationMessage[];
	    supported: boolean;
	    unavailableHint: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionConversationResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.agent = source["agent"];
	        this.projectPath = source["projectPath"];
	        this.sourcePath = source["sourcePath"];
	        this.activeDurationSeconds = source["activeDurationSeconds"];
	        this.messages = this.convertValues(source["messages"], ConversationMessage);
	        this.supported = source["supported"];
	        this.unavailableHint = source["unavailableHint"];
	    }
	
		convertValues(a: any, classs: any, asMap: boolean = false): any {
		    if (!a) {
		        return a;
		    }
		    if (a.slice && a.map) {
		        return (a as any[]).map(elem => this.convertValues(elem, classs));
		    } else if ("object" === typeof a) {
		        if (asMap) {
		            for (const key of Object.keys(a)) {
		                a[key] = new classs(a[key]);
		            }
		            return a;
		        }
		        return new classs(a);
		    }
		    return a;
		}
	}
	export class SessionPreviewRequest {
	    agent: string;
	    sessionId: string;
	    projectPath: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionPreviewRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.agent = source["agent"];
	        this.sessionId = source["sessionId"];
	        this.projectPath = source["projectPath"];
	    }
	}
	export class SessionPreviewResponse {
	    sessionId: string;
	    agent: string;
	    preview: string;
	    timestamp: string;
	    sourcePath: string;
	    activeDurationSeconds: number;
	    cached: boolean;
	    supported: boolean;
	    unavailableHint: string;
	
	    static createFrom(source: any = {}) {
	        return new SessionPreviewResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.agent = source["agent"];
	        this.preview = source["preview"];
	        this.timestamp = source["timestamp"];
	        this.sourcePath = source["sourcePath"];
	        this.activeDurationSeconds = source["activeDurationSeconds"];
	        this.cached = source["cached"];
	        this.supported = source["supported"];
	        this.unavailableHint = source["unavailableHint"];
	    }
	}

}

