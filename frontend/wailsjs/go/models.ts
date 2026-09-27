export namespace appconf {
	
	export class ProxyRule {
	    path: string;
	    target: string;
	    rewrite: string;
	    enabled: boolean;
	
	    static createFrom(source: any = {}) {
	        return new ProxyRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.target = source["target"];
	        this.rewrite = source["rewrite"];
	        this.enabled = source["enabled"];
	    }
	}
	export class BuildConfig {
	    appName: string;
	    iconPath: string;
	    distPath: string;
	    outputPath: string;
	    tempPath: string;
	    proxyRules: ProxyRule[];
	    windowWidth: number;
	    windowHeight: number;
	    windowFullscreen: boolean;
	    windowMaximized: boolean;
	    confirmClose: boolean;
	    version: string;
	    description: string;
	    company: string;
	    windowTitle: string;
	    singleInstance: boolean;
	    rememberWindow: boolean;
	    targetPlatform: string;
	    signPfxPath: string;
	    signTimestamp: string;
	
	    static createFrom(source: any = {}) {
	        return new BuildConfig(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appName = source["appName"];
	        this.iconPath = source["iconPath"];
	        this.distPath = source["distPath"];
	        this.outputPath = source["outputPath"];
	        this.tempPath = source["tempPath"];
	        this.proxyRules = this.convertValues(source["proxyRules"], ProxyRule);
	        this.windowWidth = source["windowWidth"];
	        this.windowHeight = source["windowHeight"];
	        this.windowFullscreen = source["windowFullscreen"];
	        this.windowMaximized = source["windowMaximized"];
	        this.confirmClose = source["confirmClose"];
	        this.version = source["version"];
	        this.description = source["description"];
	        this.company = source["company"];
	        this.windowTitle = source["windowTitle"];
	        this.singleInstance = source["singleInstance"];
	        this.rememberWindow = source["rememberWindow"];
	        this.targetPlatform = source["targetPlatform"];
	        this.signPfxPath = source["signPfxPath"];
	        this.signTimestamp = source["signTimestamp"];
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
	export class DistInfo {
	    path: string;
	    fileCount: number;
	    totalSize: number;
	    valid: boolean;
	    suggestedName: string;
	
	    static createFrom(source: any = {}) {
	        return new DistInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.fileCount = source["fileCount"];
	        this.totalSize = source["totalSize"];
	        this.valid = source["valid"];
	        this.suggestedName = source["suggestedName"];
	    }
	}

}

export namespace selfupdate {
	
	export class Info {
	    hasUpdate: boolean;
	    currentVersion: string;
	    latestVersion: string;
	    notesUrl: string;
	    downloadUrl: string;
	
	    static createFrom(source: any = {}) {
	        return new Info(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.hasUpdate = source["hasUpdate"];
	        this.currentVersion = source["currentVersion"];
	        this.latestVersion = source["latestVersion"];
	        this.notesUrl = source["notesUrl"];
	        this.downloadUrl = source["downloadUrl"];
	    }
	}

}

