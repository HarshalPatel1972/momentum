export namespace main {
	
	export class Activity {
	    id: string;
	    question: string;
	    options: string[];
	    client: string;
	    project: string;
	    state: string;
	    answer?: string;
	    // Go type: time
	    created: any;
	    // Go type: time
	    closed?: any;
	
	    static createFrom(source: any = {}) {
	        return new Activity(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.question = source["question"];
	        this.options = source["options"];
	        this.client = source["client"];
	        this.project = source["project"];
	        this.state = source["state"];
	        this.answer = source["answer"];
	        this.created = this.convertValues(source["created"], null);
	        this.closed = this.convertValues(source["closed"], null);
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
	export class AppState {
	    version: string;
	    configured: boolean;
	    problem: string;
	    channel: string;
	    chatName: string;
	    botUsername: string;
	    running: boolean;
	    atDesk: boolean;
	    idesLinked: number;
	    idesFound: number;
	    dataDir: string;
	
	    static createFrom(source: any = {}) {
	        return new AppState(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.version = source["version"];
	        this.configured = source["configured"];
	        this.problem = source["problem"];
	        this.channel = source["channel"];
	        this.chatName = source["chatName"];
	        this.botUsername = source["botUsername"];
	        this.running = source["running"];
	        this.atDesk = source["atDesk"];
	        this.idesLinked = source["idesLinked"];
	        this.idesFound = source["idesFound"];
	        this.dataDir = source["dataDir"];
	    }
	}
	export class IDEStatus {
	    id: string;
	    name: string;
	    installed: boolean;
	    connected: boolean;
	    stale: boolean;
	    configPath: string;
	    manual: boolean;
	    note: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new IDEStatus(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.installed = source["installed"];
	        this.connected = source["connected"];
	        this.stale = source["stale"];
	        this.configPath = source["configPath"];
	        this.manual = source["manual"];
	        this.note = source["note"];
	        this.error = source["error"];
	    }
	}
	export class TelegramChat {
	    id: string;
	    name: string;
	    bot: string;
	    error?: string;
	
	    static createFrom(source: any = {}) {
	        return new TelegramChat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.bot = source["bot"];
	        this.error = source["error"];
	    }
	}
	export class TelegramConfig {
	    bot_token: string;
	    chat_id: string;
	    chat_name?: string;
	    bot_username?: string;
	
	    static createFrom(source: any = {}) {
	        return new TelegramConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.bot_token = source["bot_token"];
	        this.chat_id = source["chat_id"];
	        this.chat_name = source["chat_name"];
	        this.bot_username = source["bot_username"];
	    }
	}

}

