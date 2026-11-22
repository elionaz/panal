export namespace forge {
	
	export class Checks {
	    state: string;
	    total: number;
	    passed: number;
	    failed: number;
	    pending: number;
	    failing?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Checks(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.total = source["total"];
	        this.passed = source["passed"];
	        this.failed = source["failed"];
	        this.pending = source["pending"];
	        this.failing = source["failing"];
	    }
	}
	export class Comment {
	    kind: string;
	    author: string;
	    body: string;
	    // Go type: time
	    at: any;
	    url?: string;
	    review?: string;
	    path?: string;
	    line?: number;
	
	    static createFrom(source: any = {}) {
	        return new Comment(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.kind = source["kind"];
	        this.author = source["author"];
	        this.body = source["body"];
	        this.at = this.convertValues(source["at"], null);
	        this.url = source["url"];
	        this.review = source["review"];
	        this.path = source["path"];
	        this.line = source["line"];
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
	export class PR {
	    number: number;
	    title: string;
	    state: string;
	    url: string;
	    base: string;
	    head: string;
	    review?: string;
	    conflicts?: boolean;
	    blocked?: boolean;
	    checks: Checks;
	
	    static createFrom(source: any = {}) {
	        return new PR(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.number = source["number"];
	        this.title = source["title"];
	        this.state = source["state"];
	        this.url = source["url"];
	        this.base = source["base"];
	        this.head = source["head"];
	        this.review = source["review"];
	        this.conflicts = source["conflicts"];
	        this.blocked = source["blocked"];
	        this.checks = this.convertValues(source["checks"], Checks);
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

export namespace gitx {
	
	export class Commit {
	    hash: string;
	    short: string;
	    subject: string;
	    author: string;
	    // Go type: time
	    date: any;
	
	    static createFrom(source: any = {}) {
	        return new Commit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hash = source["hash"];
	        this.short = source["short"];
	        this.subject = source["subject"];
	        this.author = source["author"];
	        this.date = this.convertValues(source["date"], null);
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
	export class DiffFile {
	    path: string;
	    oldPath?: string;
	    status: string;
	    additions: number;
	    deletions: number;
	    binary?: boolean;
	    untracked?: boolean;
	    patch?: string;
	    truncated?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DiffFile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.oldPath = source["oldPath"];
	        this.status = source["status"];
	        this.additions = source["additions"];
	        this.deletions = source["deletions"];
	        this.binary = source["binary"];
	        this.untracked = source["untracked"];
	        this.patch = source["patch"];
	        this.truncated = source["truncated"];
	    }
	}
	export class LossError {
	    path: string;
	    branch: string;
	    base: string;
	    uncommitted?: string[];
	    unmerged?: Commit[];
	
	    static createFrom(source: any = {}) {
	        return new LossError(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.branch = source["branch"];
	        this.base = source["base"];
	        this.uncommitted = source["uncommitted"];
	        this.unmerged = this.convertValues(source["unmerged"], Commit);
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
	export class MergeResult {
	    base: string;
	    branch: string;
	    commits: number;
	    output?: string;
	    uncommitted?: number;
	
	    static createFrom(source: any = {}) {
	        return new MergeResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.base = source["base"];
	        this.branch = source["branch"];
	        this.commits = source["commits"];
	        this.output = source["output"];
	        this.uncommitted = source["uncommitted"];
	    }
	}
	export class PRResult {
	    branch: string;
	    base: string;
	    remote: string;
	    pushed: boolean;
	    url?: string;
	    commits: number;
	    output?: string;
	    uncommitted?: number;
	
	    static createFrom(source: any = {}) {
	        return new PRResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch = source["branch"];
	        this.base = source["base"];
	        this.remote = source["remote"];
	        this.pushed = source["pushed"];
	        this.url = source["url"];
	        this.commits = source["commits"];
	        this.output = source["output"];
	        this.uncommitted = source["uncommitted"];
	    }
	}

}

export namespace history {
	
	export class Resume {
	    agent: string;
	    sessionId: string;
	
	    static createFrom(source: any = {}) {
	        return new Resume(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.agent = source["agent"];
	        this.sessionId = source["sessionId"];
	    }
	}
	export class Entry {
	    version: number;
	    id: string;
	    projectId: string;
	    projectName: string;
	    profileId: string;
	    agent: string;
	    name?: string;
	    model?: string;
	    cwd: string;
	    mode?: string;
	    worktreeId?: string;
	    branches?: string[];
	    base?: string;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    endedAt: any;
	    exitCode: number;
	    resume?: Resume;
	    prompts?: string[];
	    textBytes: number;
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.projectName = source["projectName"];
	        this.profileId = source["profileId"];
	        this.agent = source["agent"];
	        this.name = source["name"];
	        this.model = source["model"];
	        this.cwd = source["cwd"];
	        this.mode = source["mode"];
	        this.worktreeId = source["worktreeId"];
	        this.branches = source["branches"];
	        this.base = source["base"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.endedAt = this.convertValues(source["endedAt"], null);
	        this.exitCode = source["exitCode"];
	        this.resume = this.convertValues(source["resume"], Resume);
	        this.prompts = source["prompts"];
	        this.textBytes = source["textBytes"];
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
	export class Snippet {
	    source: string;
	    line: string;
	    matches: number[][];
	
	    static createFrom(source: any = {}) {
	        return new Snippet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.line = source["line"];
	        this.matches = source["matches"];
	    }
	}
	export class Hit {
	    entry: Entry;
	    snippets: Snippet[];
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new Hit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entry = this.convertValues(source["entry"], Entry);
	        this.snippets = this.convertValues(source["snippets"], Snippet);
	        this.count = source["count"];
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

export namespace mailbox {
	
	export class Message {
	    id: number;
	    projectId: string;
	    from: string;
	    fromName?: string;
	    to: string;
	    toName?: string;
	    body: string;
	    // Go type: time
	    at: any;
	    // Go type: time
	    deliveredAt?: any;
	    held?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Message(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.from = source["from"];
	        this.fromName = source["fromName"];
	        this.to = source["to"];
	        this.toName = source["toName"];
	        this.body = source["body"];
	        this.at = this.convertValues(source["at"], null);
	        this.deliveredAt = this.convertValues(source["deliveredAt"], null);
	        this.held = source["held"];
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

export namespace main {
	
	export class DaemonStatus {
	    appVersion: string;
	    daemonVersion: string;
	    mismatch: boolean;
	    newer: boolean;
	    moved?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new DaemonStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appVersion = source["appVersion"];
	        this.daemonVersion = source["daemonVersion"];
	        this.mismatch = source["mismatch"];
	        this.newer = source["newer"];
	        this.moved = source["moved"];
	    }
	}
	export class NotificationInfo {
	    state: string;
	    detail?: string;
	
	    static createFrom(source: any = {}) {
	        return new NotificationInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.detail = source["detail"];
	    }
	}
	export class OnboardingInfo {
	    notifications: string;
	    cliPath: string;
	    bundledCli: string;
	
	    static createFrom(source: any = {}) {
	        return new OnboardingInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.notifications = source["notifications"];
	        this.cliPath = source["cliPath"];
	        this.bundledCli = source["bundledCli"];
	    }
	}
	export class SettingsPatch {
	    notify?: settings.Notify;
	    activity?: settings.Activity;
	    git?: settings.Git;
	    ui?: settings.UI;
	    sessions?: settings.Sessions;
	    terminal?: settings.Terminal;
	    messages?: settings.Messages;
	    browser?: settings.Browser;
	    mcp?: settings.MCP;
	    history?: settings.History;
	
	    static createFrom(source: any = {}) {
	        return new SettingsPatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.notify = this.convertValues(source["notify"], settings.Notify);
	        this.activity = this.convertValues(source["activity"], settings.Activity);
	        this.git = this.convertValues(source["git"], settings.Git);
	        this.ui = this.convertValues(source["ui"], settings.UI);
	        this.sessions = this.convertValues(source["sessions"], settings.Sessions);
	        this.terminal = this.convertValues(source["terminal"], settings.Terminal);
	        this.messages = this.convertValues(source["messages"], settings.Messages);
	        this.browser = this.convertValues(source["browser"], settings.Browser);
	        this.mcp = this.convertValues(source["mcp"], settings.MCP);
	        this.history = this.convertValues(source["history"], settings.History);
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
	export class SpawnOptions {
	    mode: string;
	    branch: string;
	    base: string;
	    worktreeId: string;
	    trustSetup: boolean;
	    skipSetup: boolean;
	
	    static createFrom(source: any = {}) {
	        return new SpawnOptions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.mode = source["mode"];
	        this.branch = source["branch"];
	        this.base = source["base"];
	        this.worktreeId = source["worktreeId"];
	        this.trustSetup = source["trustSetup"];
	        this.skipSetup = source["skipSetup"];
	    }
	}
	export class UpdateInfo {
	    version: string;
	    ready: boolean;
	    url?: string;
	
	    static createFrom(source: any = {}) {
	        return new UpdateInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.ready = source["ready"];
	        this.url = source["url"];
	    }
	}
	export class UpdaterState {
	    enabled: boolean;
	    automaticChecks: boolean;
	    automaticDownloads: boolean;
	    lastCheck: string;
	    canCheck: boolean;
	    reason: string;
	    version: string;
	    available?: UpdateInfo;
	
	    static createFrom(source: any = {}) {
	        return new UpdaterState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.automaticChecks = source["automaticChecks"];
	        this.automaticDownloads = source["automaticDownloads"];
	        this.lastCheck = source["lastCheck"];
	        this.canCheck = source["canCheck"];
	        this.reason = source["reason"];
	        this.version = source["version"];
	        this.available = this.convertValues(source["available"], UpdateInfo);
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

export namespace profile {
	
	export class Entry {
	    id: string;
	    name?: string;
	    extends?: string;
	    enabled?: boolean;
	    order?: number;
	    cmd?: string;
	    args?: string[];
	    env?: Record<string, string>;
	    status?: string;
	    setupHooks?: string;
	    idleSeconds?: number;
	    attentionPatterns?: string[];
	    resumeArgs?: string[];
	
	    static createFrom(source: any = {}) {
	        return new Entry(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.extends = source["extends"];
	        this.enabled = source["enabled"];
	        this.order = source["order"];
	        this.cmd = source["cmd"];
	        this.args = source["args"];
	        this.env = source["env"];
	        this.status = source["status"];
	        this.setupHooks = source["setupHooks"];
	        this.idleSeconds = source["idleSeconds"];
	        this.attentionPatterns = source["attentionPatterns"];
	        this.resumeArgs = source["resumeArgs"];
	    }
	}
	export class Profile {
	    id: string;
	    name: string;
	    cmd: string;
	    args?: string[];
	    env?: Record<string, string>;
	    status: string;
	    setupHooks?: string;
	    attentionPatterns?: string[];
	    idleSeconds?: number;
	    resumeArgs?: string[];
	    source: string;
	    extends?: string;
	    enabled: boolean;
	    installed: boolean;
	    path?: string;
	    homepage?: string;
	    install?: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new Profile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.cmd = source["cmd"];
	        this.args = source["args"];
	        this.env = source["env"];
	        this.status = source["status"];
	        this.setupHooks = source["setupHooks"];
	        this.attentionPatterns = source["attentionPatterns"];
	        this.idleSeconds = source["idleSeconds"];
	        this.resumeArgs = source["resumeArgs"];
	        this.source = source["source"];
	        this.extends = source["extends"];
	        this.enabled = source["enabled"];
	        this.installed = source["installed"];
	        this.path = source["path"];
	        this.homepage = source["homepage"];
	        this.install = source["install"];
	        this.error = source["error"];
	    }
	}

}

export namespace project {
	
	export class Project {
	    id: string;
	    name: string;
	    path: string;
	    // Go type: time
	    addedAt: any;
	    color?: string;
	    icon?: string;
	    messages?: string;
	
	    static createFrom(source: any = {}) {
	        return new Project(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.path = source["path"];
	        this.addedAt = this.convertValues(source["addedAt"], null);
	        this.color = source["color"];
	        this.icon = source["icon"];
	        this.messages = source["messages"];
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

export namespace protocol {
	
	export class BatchError {
	    sessionId: string;
	    error: string;
	    errorCode?: string;
	    errorData?: number[];
	
	    static createFrom(source: any = {}) {
	        return new BatchError(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessionId = source["sessionId"];
	        this.error = source["error"];
	        this.errorCode = source["errorCode"];
	        this.errorData = source["errorData"];
	    }
	}
	export class BrowserStatus {
	    enabled: boolean;
	    headless: boolean;
	    exe: string;
	    npx: string;
	    mcp: string;
	
	    static createFrom(source: any = {}) {
	        return new BrowserStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.headless = source["headless"];
	        this.exe = source["exe"];
	        this.npx = source["npx"];
	        this.mcp = source["mcp"];
	    }
	}
	export class CatalogInfo {
	    id: string;
	    name: string;
	    binaries?: string[];
	    cmd?: string;
	    args?: string[];
	    status: string;
	    setupHooks?: string;
	    attentionPatterns?: string[];
	    resumeArgs?: string[];
	    homepage?: string;
	    install?: string;
	    command: string;
	    installed: boolean;
	    path?: string;
	
	    static createFrom(source: any = {}) {
	        return new CatalogInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.binaries = source["binaries"];
	        this.cmd = source["cmd"];
	        this.args = source["args"];
	        this.status = source["status"];
	        this.setupHooks = source["setupHooks"];
	        this.attentionPatterns = source["attentionPatterns"];
	        this.resumeArgs = source["resumeArgs"];
	        this.homepage = source["homepage"];
	        this.install = source["install"];
	        this.command = source["command"];
	        this.installed = source["installed"];
	        this.path = source["path"];
	    }
	}
	export class GitInfo {
	    files: number;
	    uncommitted: number;
	    ahead: number;
	    head?: string;
	    merged?: boolean;
	    // Go type: time
	    at: any;
	
	    static createFrom(source: any = {}) {
	        return new GitInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.files = source["files"];
	        this.uncommitted = source["uncommitted"];
	        this.ahead = source["ahead"];
	        this.head = source["head"];
	        this.merged = source["merged"];
	        this.at = this.convertValues(source["at"], null);
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
	export class DiffResult {
	    from: string;
	    files: gitx.DiffFile[];
	    additions: number;
	    deletions: number;
	    truncated?: boolean;
	    patchSize: number;
	    mode: string;
	    dir: string;
	    worktreeId?: string;
	    branch?: string;
	    startBranch?: string;
	    branches?: string[];
	    base?: string;
	    commits: gitx.Commit[];
	    git: GitInfo;
	
	    static createFrom(source: any = {}) {
	        return new DiffResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.from = source["from"];
	        this.files = this.convertValues(source["files"], gitx.DiffFile);
	        this.additions = source["additions"];
	        this.deletions = source["deletions"];
	        this.truncated = source["truncated"];
	        this.patchSize = source["patchSize"];
	        this.mode = source["mode"];
	        this.dir = source["dir"];
	        this.worktreeId = source["worktreeId"];
	        this.branch = source["branch"];
	        this.startBranch = source["startBranch"];
	        this.branches = source["branches"];
	        this.base = source["base"];
	        this.commits = this.convertValues(source["commits"], gitx.Commit);
	        this.git = this.convertValues(source["git"], GitInfo);
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
	export class ForgeListItem {
	    branch: string;
	    pr?: forge.PR;
	    current?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ForgeListItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.branch = source["branch"];
	        this.pr = this.convertValues(source["pr"], forge.PR);
	        this.current = source["current"];
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
	export class ForgeList {
	    prs: ForgeListItem[];
	    unavailable?: string;
	
	    static createFrom(source: any = {}) {
	        return new ForgeList(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.prs = this.convertValues(source["prs"], ForgeListItem);
	        this.unavailable = source["unavailable"];
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
	
	export class ForgeStatus {
	    unavailable?: string;
	    message?: string;
	    unavailableData?: any;
	    kind?: string;
	    tool?: string;
	    host?: string;
	    branch?: string;
	    base?: string;
	    pr?: forge.PR;
	    mergeMethods?: string[];
	    ahead: number;
	    title?: string;
	    body?: string;
	    session?: string;
	    result?: string;
	    warning?: string;
	
	    static createFrom(source: any = {}) {
	        return new ForgeStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.unavailable = source["unavailable"];
	        this.message = source["message"];
	        this.unavailableData = source["unavailableData"];
	        this.kind = source["kind"];
	        this.tool = source["tool"];
	        this.host = source["host"];
	        this.branch = source["branch"];
	        this.base = source["base"];
	        this.pr = this.convertValues(source["pr"], forge.PR);
	        this.mergeMethods = source["mergeMethods"];
	        this.ahead = source["ahead"];
	        this.title = source["title"];
	        this.body = source["body"];
	        this.session = source["session"];
	        this.result = source["result"];
	        this.warning = source["warning"];
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
	
	export class HandoffNote {
	    text: string;
	    shared?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HandoffNote(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.text = source["text"];
	        this.shared = source["shared"];
	    }
	}
	export class HistoryDetail {
	    entry: history.Entry;
	    text: string;
	    canResume: boolean;
	
	    static createFrom(source: any = {}) {
	        return new HistoryDetail(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.entry = this.convertValues(source["entry"], history.Entry);
	        this.text = source["text"];
	        this.canResume = source["canResume"];
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
	export class InstructionTopic {
	    slug: string;
	    title: string;
	    description?: string;
	    load: string;
	    projects?: string[];
	    enabled: boolean;
	    order: number;
	    body: string;
	
	    static createFrom(source: any = {}) {
	        return new InstructionTopic(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.slug = source["slug"];
	        this.title = source["title"];
	        this.description = source["description"];
	        this.load = source["load"];
	        this.projects = source["projects"];
	        this.enabled = source["enabled"];
	        this.order = source["order"];
	        this.body = source["body"];
	    }
	}
	export class Instructions {
	    topics: InstructionTopic[];
	    errors?: string[];
	    dir: string;
	    projectId?: string;
	    composed: string;
	    codexOwn?: boolean;
	    maxBytes: number;
	    maxTotal: number;
	
	    static createFrom(source: any = {}) {
	        return new Instructions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.topics = this.convertValues(source["topics"], InstructionTopic);
	        this.errors = source["errors"];
	        this.dir = source["dir"];
	        this.projectId = source["projectId"];
	        this.composed = source["composed"];
	        this.codexOwn = source["codexOwn"];
	        this.maxBytes = source["maxBytes"];
	        this.maxTotal = source["maxTotal"];
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
	export class MsgPeer {
	    id: string;
	    name: string;
	    profileId: string;
	    state: string;
	    activity?: string;
	    mode?: string;
	    branch?: string;
	    sameFolder?: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MsgPeer(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.profileId = source["profileId"];
	        this.state = source["state"];
	        this.activity = source["activity"];
	        this.mode = source["mode"];
	        this.branch = source["branch"];
	        this.sameFolder = source["sameFolder"];
	    }
	}
	export class MsgPeers {
	    projectId: string;
	    projectName: string;
	    self?: MsgPeer;
	    peers: MsgPeer[];
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MsgPeers(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.projectId = source["projectId"];
	        this.projectName = source["projectName"];
	        this.self = this.convertValues(source["self"], MsgPeer);
	        this.peers = this.convertValues(source["peers"], MsgPeer);
	        this.enabled = source["enabled"];
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
	export class ProjectGit {
	    isRepo: boolean;
	    root?: string;
	    currentBranch?: string;
	    defaultBase?: string;
	    branches: string[];
	    error?: string;
	    errorCode?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProjectGit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.isRepo = source["isRepo"];
	        this.root = source["root"];
	        this.currentBranch = source["currentBranch"];
	        this.defaultBase = source["defaultBase"];
	        this.branches = source["branches"];
	        this.error = source["error"];
	        this.errorCode = source["errorCode"];
	    }
	}
	export class ProjectUpdateParams {
	    id: string;
	    name?: string;
	    color?: string;
	    icon?: string;
	    messages?: string;
	
	    static createFrom(source: any = {}) {
	        return new ProjectUpdateParams(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.color = source["color"];
	        this.icon = source["icon"];
	        this.messages = source["messages"];
	    }
	}
	export class Progress {
	    state: string;
	    value: number;
	
	    static createFrom(source: any = {}) {
	        return new Progress(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.state = source["state"];
	        this.value = source["value"];
	    }
	}
	export class SessionInfo {
	    suggestCopy?: string[];
	    port?: number;
	    permission?: string;
	    id: string;
	    projectId: string;
	    projectDir: string;
	    profileId: string;
	    name?: string;
	    projectName?: string;
	    cmd: string;
	    args?: string[];
	    cwd: string;
	    pid: number;
	    // Go type: time
	    createdAt: any;
	    state: string;
	    exitCode?: number;
	    cols: number;
	    rows: number;
	    attentionKind?: string;
	    attentionMessage?: string;
	    // Go type: time
	    attentionAt?: any;
	    unread?: boolean;
	    msgPending?: number;
	    msgHold?: string;
	    model?: string;
	    browserUrl?: string;
	    stopped?: boolean;
	    continuedFrom?: string[];
	    mode?: string;
	    worktreeId?: string;
	    worktreePath?: string;
	    branch?: string;
	    base?: string;
	    git?: GitInfo;
	    // Go type: time
	    suspendedAt?: any;
	    resumeMode?: string;
	    resumeError?: string;
	    resumeErrorCode?: string;
	    activity?: string;
	    activityKind?: string;
	    activityTarget?: string;
	    // Go type: time
	    activityAt?: any;
	    activitySource?: string;
	    // Go type: time
	    turnStartedAt?: any;
	    // Go type: time
	    turnEndedAt?: any;
	    // Go type: time
	    lastOutputAt?: any;
	    // Go type: time
	    lastEventAt?: any;
	    stalled?: boolean;
	    // Go type: time
	    stalledAt?: any;
	    // Go type: Progress
	    progress?: any;
	
	    static createFrom(source: any = {}) {
	        return new SessionInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.suggestCopy = source["suggestCopy"];
	        this.port = source["port"];
	        this.permission = source["permission"];
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.projectDir = source["projectDir"];
	        this.profileId = source["profileId"];
	        this.name = source["name"];
	        this.projectName = source["projectName"];
	        this.cmd = source["cmd"];
	        this.args = source["args"];
	        this.cwd = source["cwd"];
	        this.pid = source["pid"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.state = source["state"];
	        this.exitCode = source["exitCode"];
	        this.cols = source["cols"];
	        this.rows = source["rows"];
	        this.attentionKind = source["attentionKind"];
	        this.attentionMessage = source["attentionMessage"];
	        this.attentionAt = this.convertValues(source["attentionAt"], null);
	        this.unread = source["unread"];
	        this.msgPending = source["msgPending"];
	        this.msgHold = source["msgHold"];
	        this.model = source["model"];
	        this.browserUrl = source["browserUrl"];
	        this.stopped = source["stopped"];
	        this.continuedFrom = source["continuedFrom"];
	        this.mode = source["mode"];
	        this.worktreeId = source["worktreeId"];
	        this.worktreePath = source["worktreePath"];
	        this.branch = source["branch"];
	        this.base = source["base"];
	        this.git = this.convertValues(source["git"], GitInfo);
	        this.suspendedAt = this.convertValues(source["suspendedAt"], null);
	        this.resumeMode = source["resumeMode"];
	        this.resumeError = source["resumeError"];
	        this.resumeErrorCode = source["resumeErrorCode"];
	        this.activity = source["activity"];
	        this.activityKind = source["activityKind"];
	        this.activityTarget = source["activityTarget"];
	        this.activityAt = this.convertValues(source["activityAt"], null);
	        this.activitySource = source["activitySource"];
	        this.turnStartedAt = this.convertValues(source["turnStartedAt"], null);
	        this.turnEndedAt = this.convertValues(source["turnEndedAt"], null);
	        this.lastOutputAt = this.convertValues(source["lastOutputAt"], null);
	        this.lastEventAt = this.convertValues(source["lastEventAt"], null);
	        this.stalled = source["stalled"];
	        this.stalledAt = this.convertValues(source["stalledAt"], null);
	        this.progress = this.convertValues(source["progress"], null);
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
	export class SessionBatch {
	    sessions: SessionInfo[];
	    failed?: BatchError[];
	
	    static createFrom(source: any = {}) {
	        return new SessionBatch(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sessions = this.convertValues(source["sessions"], SessionInfo);
	        this.failed = this.convertValues(source["failed"], BatchError);
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
	
	export class WorktreeCopyResult {
	    copied: string[];
	    skipped?: string[];
	    missing?: string[];
	    created?: string;
	
	    static createFrom(source: any = {}) {
	        return new WorktreeCopyResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.copied = source["copied"];
	        this.skipped = source["skipped"];
	        this.missing = source["missing"];
	        this.created = source["created"];
	    }
	}
	export class WorktreeInfo {
	    id: string;
	    projectId: string;
	    path: string;
	    branch: string;
	    base: string;
	    profileId?: string;
	    // Go type: time
	    createdAt: any;
	    sessionIds: string[];
	    live: boolean;
	    orphan: boolean;
	    git?: GitInfo;
	
	    static createFrom(source: any = {}) {
	        return new WorktreeInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.projectId = source["projectId"];
	        this.path = source["path"];
	        this.branch = source["branch"];
	        this.base = source["base"];
	        this.profileId = source["profileId"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.sessionIds = source["sessionIds"];
	        this.live = source["live"];
	        this.orphan = source["orphan"];
	        this.git = this.convertValues(source["git"], GitInfo);
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
	export class WorktreeSkipped {
	    worktree: WorktreeInfo;
	    reason: string;
	
	    static createFrom(source: any = {}) {
	        return new WorktreeSkipped(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.worktree = this.convertValues(source["worktree"], WorktreeInfo);
	        this.reason = source["reason"];
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
	export class WorktreeRemoveMergedResult {
	    removed: WorktreeInfo[];
	    skipped: WorktreeSkipped[];
	
	    static createFrom(source: any = {}) {
	        return new WorktreeRemoveMergedResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.removed = this.convertValues(source["removed"], WorktreeInfo);
	        this.skipped = this.convertValues(source["skipped"], WorktreeSkipped);
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
	export class WorktreeRemoveResult {
	    removed: boolean;
	    loss?: gitx.LossError;
	
	    static createFrom(source: any = {}) {
	        return new WorktreeRemoveResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.removed = source["removed"];
	        this.loss = this.convertValues(source["loss"], gitx.LossError);
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

export namespace settings {
	
	export class Activity {
	    stallSeconds: number;
	
	    static createFrom(source: any = {}) {
	        return new Activity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.stallSeconds = source["stallSeconds"];
	    }
	}
	export class Browser {
	    enabled: boolean;
	    headless: boolean;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new Browser(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.headless = source["headless"];
	        this.path = source["path"];
	    }
	}
	export class Git {
	    editor: string;
	    worktreeDir: string;
	    defaultMode: string;
	
	    static createFrom(source: any = {}) {
	        return new Git(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.editor = source["editor"];
	        this.worktreeDir = source["worktreeDir"];
	        this.defaultMode = source["defaultMode"];
	    }
	}
	export class History {
	    enabled: boolean;
	    keep: number;
	
	    static createFrom(source: any = {}) {
	        return new History(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.keep = source["keep"];
	    }
	}
	export class MCP {
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new MCP(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	    }
	}
	export class Messages {
	    enabled: boolean;
	    autoDeliver: boolean;
	
	    static createFrom(source: any = {}) {
	        return new Messages(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.autoDeliver = source["autoDeliver"];
	    }
	}
	export class Notify {
	    enabled: boolean;
	    sound: boolean;
	    notifyOnApproval: boolean;
	    notifyOnQuestion: boolean;
	    notifyOnDone: boolean;
	    notifyOnError: boolean;
	    notifyOnStall: boolean;
	    fallback?: string;
	
	    static createFrom(source: any = {}) {
	        return new Notify(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.sound = source["sound"];
	        this.notifyOnApproval = source["notifyOnApproval"];
	        this.notifyOnQuestion = source["notifyOnQuestion"];
	        this.notifyOnDone = source["notifyOnDone"];
	        this.notifyOnError = source["notifyOnError"];
	        this.notifyOnStall = source["notifyOnStall"];
	        this.fallback = source["fallback"];
	    }
	}
	export class Sessions {
	    restore: string;
	
	    static createFrom(source: any = {}) {
	        return new Sessions(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.restore = source["restore"];
	    }
	}
	export class Terminal {
	    fontFamily: string;
	    fontSize: number;
	    lineHeight: number;
	    cursorStyle: string;
	    theme: string;
	
	    static createFrom(source: any = {}) {
	        return new Terminal(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.fontFamily = source["fontFamily"];
	        this.fontSize = source["fontSize"];
	        this.lineHeight = source["lineHeight"];
	        this.cursorStyle = source["cursorStyle"];
	        this.theme = source["theme"];
	    }
	}
	export class UI {
	    language?: string;
	    appearance?: string;
	
	    static createFrom(source: any = {}) {
	        return new UI(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.language = source["language"];
	        this.appearance = source["appearance"];
	    }
	}
	export class Settings {
	    notify: Notify;
	    activity: Activity;
	    git: Git;
	    ui: UI;
	    sessions: Sessions;
	    terminal: Terminal;
	    messages: Messages;
	    browser: Browser;
	    mcp: MCP;
	    history: History;
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.notify = this.convertValues(source["notify"], Notify);
	        this.activity = this.convertValues(source["activity"], Activity);
	        this.git = this.convertValues(source["git"], Git);
	        this.ui = this.convertValues(source["ui"], UI);
	        this.sessions = this.convertValues(source["sessions"], Sessions);
	        this.terminal = this.convertValues(source["terminal"], Terminal);
	        this.messages = this.convertValues(source["messages"], Messages);
	        this.browser = this.convertValues(source["browser"], Browser);
	        this.mcp = this.convertValues(source["mcp"], MCP);
	        this.history = this.convertValues(source["history"], History);
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

