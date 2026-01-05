export namespace main {
	
	export class RecentChannel {
	    name: string;
	    icon: string;
	    config_key: string;
	    last_used: string;
	
	    static createFrom(source: any = {}) {
	        return new RecentChannel(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.icon = source["icon"];
	        this.config_key = source["config_key"];
	        this.last_used = source["last_used"];
	    }
	}

}

