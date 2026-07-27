export namespace ai {
	
	export class AssistRequest {
	    instruction: string;
	    contextKind: string;
	    contextText: string;
	    filePath: string;
	    appName: string;
	    appBundleId: string;
	    customPrompt: string;
	    canReplace: boolean;
	
	    static createFrom(source: any = {}) {
	        return new AssistRequest(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.instruction = source["instruction"];
	        this.contextKind = source["contextKind"];
	        this.contextText = source["contextText"];
	        this.filePath = source["filePath"];
	        this.appName = source["appName"];
	        this.appBundleId = source["appBundleId"];
	        this.customPrompt = source["customPrompt"];
	        this.canReplace = source["canReplace"];
	    }
	}
	export class AssistResult {
	    intent: string;
	    supportReport: string;
	    replacement: string;
	
	    static createFrom(source: any = {}) {
	        return new AssistResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.intent = source["intent"];
	        this.supportReport = source["supportReport"];
	        this.replacement = source["replacement"];
	    }
	}
	export class Settings {
	    enabled: boolean;
	    provider: string;
	    endpoint: string;
	    model: string;
	    apiKey: string;
	    temperature: number;
	    hotkey: string;
	    useSelectedText: boolean;
	    useSelectedFile: boolean;
	    replaceSelectedText: boolean;
	    pasteReplacementBundleIds: string[];
	
	    static createFrom(source: any = {}) {
	        return new Settings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.enabled = source["enabled"];
	        this.provider = source["provider"];
	        this.endpoint = source["endpoint"];
	        this.model = source["model"];
	        this.apiKey = source["apiKey"];
	        this.temperature = source["temperature"];
	        this.hotkey = source["hotkey"];
	        this.useSelectedText = source["useSelectedText"];
	        this.useSelectedFile = source["useSelectedFile"];
	        this.replaceSelectedText = source["replaceSelectedText"];
	        this.pasteReplacementBundleIds = source["pasteReplacementBundleIds"];
	    }
	}

}

export namespace main {
	
	export class AIPromptProfile {
	    id: string;
	    appName: string;
	    appBundleId: string;
	    useSelectedText: boolean;
	    runWithoutSelection: boolean;
	    selectedTextPrompt: string;
	    noSelectionPrompt: string;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new AIPromptProfile(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.appName = source["appName"];
	        this.appBundleId = source["appBundleId"];
	        this.useSelectedText = source["useSelectedText"];
	        this.runWithoutSelection = source["runWithoutSelection"];
	        this.selectedTextPrompt = source["selectedTextPrompt"];
	        this.noSelectionPrompt = source["noSelectionPrompt"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class AIPromptProfileInput {
	    appName: string;
	    appBundleId: string;
	    useSelectedText: boolean;
	    runWithoutSelection: boolean;
	    selectedTextPrompt: string;
	    noSelectionPrompt: string;
	
	    static createFrom(source: any = {}) {
	        return new AIPromptProfileInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.appName = source["appName"];
	        this.appBundleId = source["appBundleId"];
	        this.useSelectedText = source["useSelectedText"];
	        this.runWithoutSelection = source["runWithoutSelection"];
	        this.selectedTextPrompt = source["selectedTextPrompt"];
	        this.noSelectionPrompt = source["noSelectionPrompt"];
	    }
	}
	export class AIPromptRule {
	    useSelectedText: boolean;
	    runWithoutSelection: boolean;
	    selectedTextPrompt: string;
	    noSelectionPrompt: string;
	
	    static createFrom(source: any = {}) {
	        return new AIPromptRule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.useSelectedText = source["useSelectedText"];
	        this.runWithoutSelection = source["runWithoutSelection"];
	        this.selectedTextPrompt = source["selectedTextPrompt"];
	        this.noSelectionPrompt = source["noSelectionPrompt"];
	    }
	}
	export class AIPromptSettings {
	    common: AIPromptRule;
	    profiles: AIPromptProfile[];
	
	    static createFrom(source: any = {}) {
	        return new AIPromptSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.common = this.convertValues(source["common"], AIPromptRule);
	        this.profiles = this.convertValues(source["profiles"], AIPromptProfile);
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
	export class GeneralSettings {
	    themeMode: string;
	    language: string;
	    typingTrendEnabled: boolean;
	    startAtLogin: boolean;
	    soundName: string;
	
	    static createFrom(source: any = {}) {
	        return new GeneralSettings(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.themeMode = source["themeMode"];
	        this.language = source["language"];
	        this.typingTrendEnabled = source["typingTrendEnabled"];
	        this.startAtLogin = source["startAtLogin"];
	        this.soundName = source["soundName"];
	    }
	}

}

export namespace platform {
	
	export class AppInfo {
	    name: string;
	    bundleId: string;
	    path: string;
	
	    static createFrom(source: any = {}) {
	        return new AppInfo(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.bundleId = source["bundleId"];
	        this.path = source["path"];
	    }
	}
	export class Status {
	    accessibilityTrusted: boolean;
	    secureInputActive: boolean;
	    activeAppName: string;
	    activeBundleId: string;
	    flowEngineRunning: boolean;
	    message: string;
	
	    static createFrom(source: any = {}) {
	        return new Status(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.accessibilityTrusted = source["accessibilityTrusted"];
	        this.secureInputActive = source["secureInputActive"];
	        this.activeAppName = source["activeAppName"];
	        this.activeBundleId = source["activeBundleId"];
	        this.flowEngineRunning = source["flowEngineRunning"];
	        this.message = source["message"];
	    }
	}

}

export namespace storage {
	
	export class DailyTypingStat {
	    date: string;
	    count: number;
	
	    static createFrom(source: any = {}) {
	        return new DailyTypingStat(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.date = source["date"];
	        this.count = source["count"];
	    }
	}
	export class Snippet {
	    id: number;
	    labelId: number;
	    shortcut: string;
	    title: string;
	    content: string;
	    contentType: string;
	    enabled: boolean;
	    caseSensitive: boolean;
	    usePaste: boolean;
	    expandMode: string;
	    usageCount: number;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Snippet(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.labelId = source["labelId"];
	        this.shortcut = source["shortcut"];
	        this.title = source["title"];
	        this.content = source["content"];
	        this.contentType = source["contentType"];
	        this.enabled = source["enabled"];
	        this.caseSensitive = source["caseSensitive"];
	        this.usePaste = source["usePaste"];
	        this.expandMode = source["expandMode"];
	        this.usageCount = source["usageCount"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class DashboardStats {
	    totalExpansions: number;
	    todayExpansions: number;
	    snippetCount: number;
	    enabledCount: number;
	    todayTypingCount: number;
	    averageDailyTyping: number;
	    typingHistory: DailyTypingStat[];
	    topSnippets: Snippet[];
	
	    static createFrom(source: any = {}) {
	        return new DashboardStats(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.totalExpansions = source["totalExpansions"];
	        this.todayExpansions = source["todayExpansions"];
	        this.snippetCount = source["snippetCount"];
	        this.enabledCount = source["enabledCount"];
	        this.todayTypingCount = source["todayTypingCount"];
	        this.averageDailyTyping = source["averageDailyTyping"];
	        this.typingHistory = this.convertValues(source["typingHistory"], DailyTypingStat);
	        this.topSnippets = this.convertValues(source["topSnippets"], Snippet);
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
	export class Label {
	    id: number;
	    name: string;
	    description: string;
	    color: string;
	    snippetCount: number;
	    enabledCount: number;
	    createdAt: string;
	    updatedAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Label(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.color = source["color"];
	        this.snippetCount = source["snippetCount"];
	        this.enabledCount = source["enabledCount"];
	        this.createdAt = source["createdAt"];
	        this.updatedAt = source["updatedAt"];
	    }
	}
	export class LabelInput {
	    name: string;
	    description: string;
	    color: string;
	
	    static createFrom(source: any = {}) {
	        return new LabelInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.description = source["description"];
	        this.color = source["color"];
	    }
	}
	
	export class SnippetInput {
	    labelId: number;
	    shortcut: string;
	    title: string;
	    content: string;
	    contentType: string;
	    enabled: boolean;
	    caseSensitive: boolean;
	    usePaste: boolean;
	    expandMode: string;
	
	    static createFrom(source: any = {}) {
	        return new SnippetInput(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.labelId = source["labelId"];
	        this.shortcut = source["shortcut"];
	        this.title = source["title"];
	        this.content = source["content"];
	        this.contentType = source["contentType"];
	        this.enabled = source["enabled"];
	        this.caseSensitive = source["caseSensitive"];
	        this.usePaste = source["usePaste"];
	        this.expandMode = source["expandMode"];
	    }
	}

}
