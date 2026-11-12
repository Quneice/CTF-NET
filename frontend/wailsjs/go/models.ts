export namespace aiagent {
	
	export class UsageTotal {
	    in: number;
	    out: number;
	
	    static createFrom(source: any = {}) {
	        return new UsageTotal(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.in = source["in"];
	        this.out = source["out"];
	    }
	}
	export class AITaskSummary {
	    id: string;
	    description: string;
	    goal_text: string;
	    status: string;
	    summary?: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    completed_at?: any;
	    paused: boolean;
	    intents: number;
	    open_intents: number;
	    findings: number;
	    assets: number;
	    facts: number;
	    usage: UsageTotal;
	
	    static createFrom(source: any = {}) {
	        return new AITaskSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.description = source["description"];
	        this.goal_text = source["goal_text"];
	        this.status = source["status"];
	        this.summary = source["summary"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.completed_at = this.convertValues(source["completed_at"], null);
	        this.paused = source["paused"];
	        this.intents = source["intents"];
	        this.open_intents = source["open_intents"];
	        this.findings = source["findings"];
	        this.assets = source["assets"];
	        this.facts = source["facts"];
	        this.usage = this.convertValues(source["usage"], UsageTotal);
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
	export class Activity {
	    seq: number;
	    // Go type: time
	    ts: any;
	    agent: string;
	    kind: string;
	    summary: string;
	
	    static createFrom(source: any = {}) {
	        return new Activity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.seq = source["seq"];
	        this.ts = this.convertValues(source["ts"], null);
	        this.agent = source["agent"];
	        this.kind = source["kind"];
	        this.summary = source["summary"];
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
	export class ActivityDTO {
	    task_id: string;
	    task_desc: string;
	    activity: Activity;
	
	    static createFrom(source: any = {}) {
	        return new ActivityDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.task_id = source["task_id"];
	        this.task_desc = source["task_desc"];
	        this.activity = this.convertValues(source["activity"], Activity);
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
	export class Asset {
	    id: number;
	    type: string;
	    url?: string;
	    domain?: string;
	    ip?: string;
	    props?: Record<string, string>;
	    added_by?: string;
	
	    static createFrom(source: any = {}) {
	        return new Asset(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.type = source["type"];
	        this.url = source["url"];
	        this.domain = source["domain"];
	        this.ip = source["ip"];
	        this.props = source["props"];
	        this.added_by = source["added_by"];
	    }
	}
	export class AssetDTO {
	    task_id: string;
	    task_desc: string;
	    asset: Asset;
	
	    static createFrom(source: any = {}) {
	        return new AssetDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.task_id = source["task_id"];
	        this.task_desc = source["task_desc"];
	        this.asset = this.convertValues(source["asset"], Asset);
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
	export class ChatSeg {
	    kind: string;
	    text?: string;
	    name?: string;
	    summary?: string;
	    is_error?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ChatSeg(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.text = source["text"];
	        this.name = source["name"];
	        this.summary = source["summary"];
	        this.is_error = source["is_error"];
	    }
	}
	export class ChatMsg {
	    role: string;
	    content: string;
	    reasoning?: string;
	    segments?: ChatSeg[];
	    ts: string;
	
	    static createFrom(source: any = {}) {
	        return new ChatMsg(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.role = source["role"];
	        this.content = source["content"];
	        this.reasoning = source["reasoning"];
	        this.segments = this.convertValues(source["segments"], ChatSeg);
	        this.ts = source["ts"];
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
	
	export class ChatSession {
	    id: string;
	    title: string;
	    created_at: string;
	    updated_at: string;
	    messages: ChatMsg[];
	    task_ids?: string[];
	    task_dirs?: Record<string, string>;
	
	    static createFrom(source: any = {}) {
	        return new ChatSession(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.messages = this.convertValues(source["messages"], ChatMsg);
	        this.task_ids = source["task_ids"];
	        this.task_dirs = source["task_dirs"];
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
	export class ChatSessionSummary {
	    id: string;
	    title: string;
	    updated_at: string;
	    msg_count: number;
	
	    static createFrom(source: any = {}) {
	        return new ChatSessionSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.updated_at = source["updated_at"];
	        this.msg_count = source["msg_count"];
	    }
	}
	export class DayUsage {
	    day: string;
	    usage: UsageTotal;
	
	    static createFrom(source: any = {}) {
	        return new DayUsage(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.day = source["day"];
	        this.usage = this.convertValues(source["usage"], UsageTotal);
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
	export class Edge {
	    id: number;
	    from: string;
	    to: string;
	    rel: string;
	
	    static createFrom(source: any = {}) {
	        return new Edge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.from = source["from"];
	        this.to = source["to"];
	        this.rel = source["rel"];
	    }
	}
	export class Fact {
	    id: number;
	    text: string;
	    kind?: string;
	    source?: string;
	
	    static createFrom(source: any = {}) {
	        return new Fact(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.text = source["text"];
	        this.kind = source["kind"];
	        this.source = source["source"];
	    }
	}
	export class Finding {
	    id: number;
	    title: string;
	    vuln_class: string;
	    severity: string;
	    summary: string;
	    evidence: string;
	    poc?: string;
	    asset_ids?: number[];
	    intent_id?: number;
	    confirmed: boolean;
	    created_at: string;
	
	    static createFrom(source: any = {}) {
	        return new Finding(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.vuln_class = source["vuln_class"];
	        this.severity = source["severity"];
	        this.summary = source["summary"];
	        this.evidence = source["evidence"];
	        this.poc = source["poc"];
	        this.asset_ids = source["asset_ids"];
	        this.intent_id = source["intent_id"];
	        this.confirmed = source["confirmed"];
	        this.created_at = source["created_at"];
	    }
	}
	export class FindingDTO {
	    task_id: string;
	    task_desc: string;
	    finding: Finding;
	
	    static createFrom(source: any = {}) {
	        return new FindingDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.task_id = source["task_id"];
	        this.task_desc = source["task_desc"];
	        this.finding = this.convertValues(source["finding"], Finding);
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
	export class Goal {
	    id: number;
	    text: string;
	    met: boolean;
	    met_by?: string;
	    evd_sum?: string;
	    when?: string;
	
	    static createFrom(source: any = {}) {
	        return new Goal(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.text = source["text"];
	        this.met = source["met"];
	        this.met_by = source["met_by"];
	        this.evd_sum = source["evd_sum"];
	        this.when = source["when"];
	    }
	}
	export class Intent {
	    id: number;
	    summary: string;
	    state: string;
	    parent_ids?: number[];
	    asset_ids?: number[];
	    result?: string;
	    steer?: string;
	    trace?: string[];
	    created_at: string;
	    updated_at?: string;
	    round: number;
	    worker?: string;
	
	    static createFrom(source: any = {}) {
	        return new Intent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.summary = source["summary"];
	        this.state = source["state"];
	        this.parent_ids = source["parent_ids"];
	        this.asset_ids = source["asset_ids"];
	        this.result = source["result"];
	        this.steer = source["steer"];
	        this.trace = source["trace"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.round = source["round"];
	        this.worker = source["worker"];
	    }
	}
	export class StatsOverview {
	    tasks: number;
	    running: number;
	    paused: number;
	    done: number;
	    exhausted: number;
	    failed: number;
	    findings: number;
	    assets: number;
	    facts: number;
	    intents: number;
	    usage: UsageTotal;
	    by_day: DayUsage[];
	
	    static createFrom(source: any = {}) {
	        return new StatsOverview(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.tasks = source["tasks"];
	        this.running = source["running"];
	        this.paused = source["paused"];
	        this.done = source["done"];
	        this.exhausted = source["exhausted"];
	        this.failed = source["failed"];
	        this.findings = source["findings"];
	        this.assets = source["assets"];
	        this.facts = source["facts"];
	        this.intents = source["intents"];
	        this.usage = this.convertValues(source["usage"], UsageTotal);
	        this.by_day = this.convertValues(source["by_day"], DayUsage);
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
	export class Task {
	    id: string;
	    description: string;
	    goal_text: string;
	    hint?: string;
	    constraints?: string;
	    scope?: string;
	    status: string;
	    summary?: string;
	    // Go type: time
	    created_at: any;
	    // Go type: time
	    started_at: any;
	    // Go type: time
	    completed_at?: any;
	    timeout_sec: number;
	    max_turns: number;
	    max_workers: number;
	    plan_rounds: number;
	    rounds_used: number;
	    enable_exec: boolean;
	    goals: Goal[];
	    intents: Intent[];
	    assets: Asset[];
	    facts: Fact[];
	    findings: Finding[];
	    activities: Activity[];
	    edges?: Edge[];
	    usage: UsageTotal;
	    usage_by_day?: DayUsage[];
	    paused: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Task(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.description = source["description"];
	        this.goal_text = source["goal_text"];
	        this.hint = source["hint"];
	        this.constraints = source["constraints"];
	        this.scope = source["scope"];
	        this.status = source["status"];
	        this.summary = source["summary"];
	        this.created_at = this.convertValues(source["created_at"], null);
	        this.started_at = this.convertValues(source["started_at"], null);
	        this.completed_at = this.convertValues(source["completed_at"], null);
	        this.timeout_sec = source["timeout_sec"];
	        this.max_turns = source["max_turns"];
	        this.max_workers = source["max_workers"];
	        this.plan_rounds = source["plan_rounds"];
	        this.rounds_used = source["rounds_used"];
	        this.enable_exec = source["enable_exec"];
	        this.goals = this.convertValues(source["goals"], Goal);
	        this.intents = this.convertValues(source["intents"], Intent);
	        this.assets = this.convertValues(source["assets"], Asset);
	        this.facts = this.convertValues(source["facts"], Fact);
	        this.findings = this.convertValues(source["findings"], Finding);
	        this.activities = this.convertValues(source["activities"], Activity);
	        this.edges = this.convertValues(source["edges"], Edge);
	        this.usage = this.convertValues(source["usage"], UsageTotal);
	        this.usage_by_day = this.convertValues(source["usage_by_day"], DayUsage);
	        this.paused = source["paused"];
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

}

export namespace core {
	
	export class AiConfig {
	    api_key: string;
	    base_url: string;
	    model: string;
	    reasoning_effort: string;
	    max_turns: number;
	    max_workers: number;
	    plan_rounds: number;
	    timeout_seconds: number;
	    enable_exec: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AiConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.api_key = source["api_key"];
	        this.base_url = source["base_url"];
	        this.model = source["model"];
	        this.reasoning_effort = source["reasoning_effort"];
	        this.max_turns = source["max_turns"];
	        this.max_workers = source["max_workers"];
	        this.plan_rounds = source["plan_rounds"];
	        this.timeout_seconds = source["timeout_seconds"];
	        this.enable_exec = source["enable_exec"];
	    }
	}
	export class ProxyConfig {
	    url: string;
	
	    static createFrom(source: any = {}) {
	        return new ProxyConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.url = source["url"];
	    }
	}
	export class ExecutorConfig {
	    default_timeout: number;
	    max_timeout: number;
	    backend: string;
	    docker_image: string;
	    docker_network: string;
	    docker_data_volume: string;
	
	    static createFrom(source: any = {}) {
	        return new ExecutorConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.default_timeout = source["default_timeout"];
	        this.max_timeout = source["max_timeout"];
	        this.backend = source["backend"];
	        this.docker_image = source["docker_image"];
	        this.docker_network = source["docker_network"];
	        this.docker_data_volume = source["docker_data_volume"];
	    }
	}
	export class WordlistsConfig {
	    dir_enum: string;
	    password: string;
	    username: string;
	    subdomain: string;
	
	    static createFrom(source: any = {}) {
	        return new WordlistsConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.dir_enum = source["dir_enum"];
	        this.password = source["password"];
	        this.username = source["username"];
	        this.subdomain = source["subdomain"];
	    }
	}
	export class ToolsConfig {
	    python3: string;
	    java: string;
	    javac: string;
	    nmap: string;
	    nuclei: string;
	    xray: string;
	    sqlmap: string;
	    putty_dir: string;
	    fscan: string;
	
	    static createFrom(source: any = {}) {
	        return new ToolsConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.python3 = source["python3"];
	        this.java = source["java"];
	        this.javac = source["javac"];
	        this.nmap = source["nmap"];
	        this.nuclei = source["nuclei"];
	        this.xray = source["xray"];
	        this.sqlmap = source["sqlmap"];
	        this.putty_dir = source["putty_dir"];
	        this.fscan = source["fscan"];
	    }
	}
	export class FrpConfig {
	    ssh_host: string;
	    ssh_port: number;
	    ssh_user: string;
	    ssh_pass: string;
	    remote_dir: string;
	    bind_port: number;
	    token: string;
	    vhost_http_port: number;
	    vhost_https_port: number;
	    dashboard_port: number;
	    dashboard_user: string;
	    dashboard_pass: string;
	    frps_version: string;
	    download_url: string;
	
	    static createFrom(source: any = {}) {
	        return new FrpConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ssh_host = source["ssh_host"];
	        this.ssh_port = source["ssh_port"];
	        this.ssh_user = source["ssh_user"];
	        this.ssh_pass = source["ssh_pass"];
	        this.remote_dir = source["remote_dir"];
	        this.bind_port = source["bind_port"];
	        this.token = source["token"];
	        this.vhost_http_port = source["vhost_http_port"];
	        this.vhost_https_port = source["vhost_https_port"];
	        this.dashboard_port = source["dashboard_port"];
	        this.dashboard_user = source["dashboard_user"];
	        this.dashboard_pass = source["dashboard_pass"];
	        this.frps_version = source["frps_version"];
	        this.download_url = source["download_url"];
	    }
	}
	export class JndiConfig {
	    ssh_host: string;
	    ssh_port: number;
	    ssh_user: string;
	    ssh_pass: string;
	    lhost: string;
	    ldap_port: number;
	    rmi_port: number;
	    http_port: number;
	    lport: number;
	    gadget_class: string;
	    shell: string;
	
	    static createFrom(source: any = {}) {
	        return new JndiConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ssh_host = source["ssh_host"];
	        this.ssh_port = source["ssh_port"];
	        this.ssh_user = source["ssh_user"];
	        this.ssh_pass = source["ssh_pass"];
	        this.lhost = source["lhost"];
	        this.ldap_port = source["ldap_port"];
	        this.rmi_port = source["rmi_port"];
	        this.http_port = source["http_port"];
	        this.lport = source["lport"];
	        this.gadget_class = source["gadget_class"];
	        this.shell = source["shell"];
	    }
	}
	export class AppConfig {
	    jndi: JndiConfig;
	    frp: FrpConfig;
	    tools: ToolsConfig;
	    wordlists: WordlistsConfig;
	    executor: ExecutorConfig;
	    proxy: ProxyConfig;
	    ai: AiConfig;
	
	    static createFrom(source: any = {}) {
	        return new AppConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.jndi = this.convertValues(source["jndi"], JndiConfig);
	        this.frp = this.convertValues(source["frp"], FrpConfig);
	        this.tools = this.convertValues(source["tools"], ToolsConfig);
	        this.wordlists = this.convertValues(source["wordlists"], WordlistsConfig);
	        this.executor = this.convertValues(source["executor"], ExecutorConfig);
	        this.proxy = this.convertValues(source["proxy"], ProxyConfig);
	        this.ai = this.convertValues(source["ai"], AiConfig);
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
	
	
	
	
	
	export class WordlistInfo {
	    name: string;
	    lines: number;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new WordlistInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.lines = source["lines"];
	        this.size = source["size"];
	    }
	}

}

export namespace main {
	
	export class CategoryInfo {
	    name: string;
	    tool_count: number;
	
	    static createFrom(source: any = {}) {
	        return new CategoryInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.tool_count = source["tool_count"];
	    }
	}
	export class ExecResponse {
	    output: string;
	    timed_out: boolean;
	    closed: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ExecResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.output = source["output"];
	        this.timed_out = source["timed_out"];
	        this.closed = source["closed"];
	    }
	}
	export class FileInfo {
	    file_id: string;
	    name: string;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new FileInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.file_id = source["file_id"];
	        this.name = source["name"];
	        this.size = source["size"];
	    }
	}
	export class ParamSchema {
	    name: string;
	    type: string;
	    required: boolean;
	    default: any;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new ParamSchema(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.type = source["type"];
	        this.required = source["required"];
	        this.default = source["default"];
	        this.description = source["description"];
	    }
	}
	export class RunResponse {
	    job_id: string;
	
	    static createFrom(source: any = {}) {
	        return new RunResponse(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.job_id = source["job_id"];
	    }
	}
	export class ToolDetail {
	    id: string;
	    name: string;
	    category: string;
	    version: string;
	    description: string;
	    timeout: number;
	    params: ParamSchema[];
	
	    static createFrom(source: any = {}) {
	        return new ToolDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.timeout = source["timeout"];
	        this.params = this.convertValues(source["params"], ParamSchema);
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
	export class ToolSummary {
	    id: string;
	    name: string;
	    category: string;
	    version: string;
	    description: string;
	    timeout: number;
	
	    static createFrom(source: any = {}) {
	        return new ToolSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.timeout = source["timeout"];
	    }
	}

}

export namespace pentest {
	
	export class FPVulnRow {
	    id: string;
	    name: string;
	    severity: string;
	    affected_versions: string;
	    exploit_path: string;
	    references: string[];
	
	    static createFrom(source: any = {}) {
	        return new FPVulnRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.severity = source["severity"];
	        this.affected_versions = source["affected_versions"];
	        this.exploit_path = source["exploit_path"];
	        this.references = source["references"];
	    }
	}
	export class FPPathRow {
	    path: string;
	    method: string;
	    status_code_text: string;
	    confidence?: number;
	    response_pattern: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new FPPathRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.method = source["method"];
	        this.status_code_text = source["status_code_text"];
	        this.confidence = source["confidence"];
	        this.response_pattern = source["response_pattern"];
	        this.description = source["description"];
	    }
	}
	export class FPHashRow {
	    path: string;
	    hash: string;
	    algorithm: string;
	    confidence?: number;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new FPHashRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.hash = source["hash"];
	        this.algorithm = source["algorithm"];
	        this.confidence = source["confidence"];
	        this.description = source["description"];
	    }
	}
	export class FPStaticRow {
	    pattern: string;
	    location: string;
	    confidence?: number;
	    is_regex: boolean;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new FPStaticRow(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.pattern = source["pattern"];
	        this.location = source["location"];
	        this.confidence = source["confidence"];
	        this.is_regex = source["is_regex"];
	        this.description = source["description"];
	    }
	}
	export class FPCmsDetail {
	    cms_id: string;
	    name: string;
	    category: string;
	    static_strings: FPStaticRow[];
	    file_hashes: FPHashRow[];
	    paths: FPPathRow[];
	    vulnerabilities: FPVulnRow[];
	
	    static createFrom(source: any = {}) {
	        return new FPCmsDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cms_id = source["cms_id"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.static_strings = this.convertValues(source["static_strings"], FPStaticRow);
	        this.file_hashes = this.convertValues(source["file_hashes"], FPHashRow);
	        this.paths = this.convertValues(source["paths"], FPPathRow);
	        this.vulnerabilities = this.convertValues(source["vulnerabilities"], FPVulnRow);
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
	export class FPCmsSummary {
	    cms_id: string;
	    name: string;
	    category: string;
	    static_count: number;
	    hash_count: number;
	    path_count: number;
	    vuln_count: number;
	
	    static createFrom(source: any = {}) {
	        return new FPCmsSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.cms_id = source["cms_id"];
	        this.name = source["name"];
	        this.category = source["category"];
	        this.static_count = source["static_count"];
	        this.hash_count = source["hash_count"];
	        this.path_count = source["path_count"];
	        this.vuln_count = source["vuln_count"];
	    }
	}
	export class FPDBListResult {
	    total: number;
	    items: FPCmsSummary[];
	
	    static createFrom(source: any = {}) {
	        return new FPDBListResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.items = this.convertValues(source["items"], FPCmsSummary);
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
	export class FPDBStats {
	    managed: boolean;
	    path: string;
	    version: string;
	    description: string;
	    file_size: number;
	    modified: number;
	    cms_count: number;
	    static_count: number;
	    hash_count: number;
	    path_count: number;
	    vuln_count: number;
	    backup_dir: string;
	    backup_count: number;
	
	    static createFrom(source: any = {}) {
	        return new FPDBStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.managed = source["managed"];
	        this.path = source["path"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.file_size = source["file_size"];
	        this.modified = source["modified"];
	        this.cms_count = source["cms_count"];
	        this.static_count = source["static_count"];
	        this.hash_count = source["hash_count"];
	        this.path_count = source["path_count"];
	        this.vuln_count = source["vuln_count"];
	        this.backup_dir = source["backup_dir"];
	        this.backup_count = source["backup_count"];
	    }
	}
	
	export class FPImportResult {
	    total: number;
	    added: number;
	    overwritten: number;
	    skipped: number;
	    skipped_reasons: string[];
	    backup_path: string;
	
	    static createFrom(source: any = {}) {
	        return new FPImportResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.total = source["total"];
	        this.added = source["added"];
	        this.overwritten = source["overwritten"];
	        this.skipped = source["skipped"];
	        this.skipped_reasons = source["skipped_reasons"];
	        this.backup_path = source["backup_path"];
	    }
	}
	
	

}

export namespace projects {
	
	export class Entry {
	    id: string;
	    project_id: string;
	    kind: string;
	    title: string;
	    content: string;
	    tags: string;
	    created_at: number;
	    updated_at: number;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.project_id = source["project_id"];
	        this.kind = source["kind"];
	        this.title = source["title"];
	        this.content = source["content"];
	        this.tags = source["tags"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	    }
	}
	export class Project {
	    id: string;
	    name: string;
	    target: string;
	    scope: string;
	    status: string;
	    description: string;
	    created_at: number;
	    updated_at: number;
	    entry_counts?: Record<string, number>;
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.target = source["target"];
	        this.scope = source["scope"];
	        this.status = source["status"];
	        this.description = source["description"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	        this.entry_counts = source["entry_counts"];
	    }
	}
	export class ProjectInput {
	    name: string;
	    target: string;
	    scope: string;
	    status: string;
	    description: string;
	
	    static createFrom(source: any = {}) {
	        return new ProjectInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.target = source["target"];
	        this.scope = source["scope"];
	        this.status = source["status"];
	        this.description = source["description"];
	    }
	}

}

export namespace vulnlib {
	
	export class AssetData {
	    mime: string;
	    data_b64: string;
	    size: number;
	    abs_path: string;
	
	    static createFrom(source: any = {}) {
	        return new AssetData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mime = source["mime"];
	        this.data_b64 = source["data_b64"];
	        this.size = source["size"];
	        this.abs_path = source["abs_path"];
	    }
	}
	export class CaseDetail {
	    id: string;
	    cat_id: string;
	    cat_name: string;
	    sys: string;
	    sys_code: string;
	    name: string;
	    title: string;
	    cves: string[];
	    payload_count: number;
	    size: number;
	    content: string;
	    payloads: string[];
	    curls: string[];
	    path: string;
	    mtime: number;
	
	    static createFrom(source: any = {}) {
	        return new CaseDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.cat_id = source["cat_id"];
	        this.cat_name = source["cat_name"];
	        this.sys = source["sys"];
	        this.sys_code = source["sys_code"];
	        this.name = source["name"];
	        this.title = source["title"];
	        this.cves = source["cves"];
	        this.payload_count = source["payload_count"];
	        this.size = source["size"];
	        this.content = source["content"];
	        this.payloads = source["payloads"];
	        this.curls = source["curls"];
	        this.path = source["path"];
	        this.mtime = source["mtime"];
	    }
	}
	export class CaseSummary {
	    id: string;
	    cat_id: string;
	    cat_name: string;
	    sys: string;
	    sys_code: string;
	    name: string;
	    title: string;
	    cves: string[];
	    payload_count: number;
	    size: number;
	
	    static createFrom(source: any = {}) {
	        return new CaseSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.cat_id = source["cat_id"];
	        this.cat_name = source["cat_name"];
	        this.sys = source["sys"];
	        this.sys_code = source["sys_code"];
	        this.name = source["name"];
	        this.title = source["title"];
	        this.cves = source["cves"];
	        this.payload_count = source["payload_count"];
	        this.size = source["size"];
	    }
	}
	export class SystemSummary {
	    name: string;
	    code: string;
	    case_count: number;
	
	    static createFrom(source: any = {}) {
	        return new SystemSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.code = source["code"];
	        this.case_count = source["case_count"];
	    }
	}
	export class Category {
	    id: string;
	    code: string;
	    name: string;
	    case_count: number;
	    systems: SystemSummary[];
	
	    static createFrom(source: any = {}) {
	        return new Category(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.code = source["code"];
	        this.name = source["name"];
	        this.case_count = source["case_count"];
	        this.systems = this.convertValues(source["systems"], SystemSummary);
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
	export class ScanSummary {
	    root_dir: string;
	    total: number;
	    updated: number;
	    unchanged: number;
	    deleted: number;
	    skipped: number;
	    errors: string[];
	    duration_ms: number;
	
	    static createFrom(source: any = {}) {
	        return new ScanSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.root_dir = source["root_dir"];
	        this.total = source["total"];
	        this.updated = source["updated"];
	        this.unchanged = source["unchanged"];
	        this.deleted = source["deleted"];
	        this.skipped = source["skipped"];
	        this.errors = source["errors"];
	        this.duration_ms = source["duration_ms"];
	    }
	}
	export class Stats {
	    scanned: boolean;
	    root_dir: string;
	    case_count: number;
	    cat_count: number;
	    last_scan: number;
	
	    static createFrom(source: any = {}) {
	        return new Stats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.scanned = source["scanned"];
	        this.root_dir = source["root_dir"];
	        this.case_count = source["case_count"];
	        this.cat_count = source["cat_count"];
	        this.last_scan = source["last_scan"];
	    }
	}

}

export namespace wiki {
	
	export class Entry {
	    id: string;
	    category: string;
	    title: string;
	    content: string;
	    created_at: number;
	    updated_at: number;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.category = source["category"];
	        this.title = source["title"];
	        this.content = source["content"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
	    }
	}

}

