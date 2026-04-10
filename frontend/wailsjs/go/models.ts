export namespace bridge {
	
	export class AISettingsDTO {
	    provider: string;
	    model: string;
	    endpoint: string;
	    apiKey: string;
	    temperature: number;
	    embeddingsModel: string;
	    disableEmbeddings: boolean;
	    systemPrompt: string;
	    semanticTopK: number;
	
	    static createFrom(source: any = {}) {
	        return new AISettingsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.provider = source["provider"];
	        this.model = source["model"];
	        this.endpoint = source["endpoint"];
	        this.apiKey = source["apiKey"];
	        this.temperature = source["temperature"];
	        this.embeddingsModel = source["embeddingsModel"];
	        this.disableEmbeddings = source["disableEmbeddings"];
	        this.systemPrompt = source["systemPrompt"];
	        this.semanticTopK = source["semanticTopK"];
	    }
	}
	export class PageBlock {
	    id: string;
	    parentId: string;
	    content: string;
	    properties?: Record<string, string>;
	    metadata: domain.BlockMetadata;
	    children: PageBlock[];
	
	    static createFrom(source: any = {}) {
	        return new PageBlock(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.parentId = source["parentId"];
	        this.content = source["content"];
	        this.properties = source["properties"];
	        this.metadata = this.convertValues(source["metadata"], domain.BlockMetadata);
	        this.children = this.convertValues(source["children"], PageBlock);
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
	export class SemanticRelatedDTO {
	    blockId: string;
	    sourcePath: string;
	    relPath: string;
	    preview: string;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new SemanticRelatedDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.blockId = source["blockId"];
	        this.sourcePath = source["sourcePath"];
	        this.relPath = source["relPath"];
	        this.preview = source["preview"];
	        this.score = source["score"];
	    }
	}
	export class SyncSettingsDTO {
	    webdavUrl: string;
	    webdavUser: string;
	    webdavPassword: string;
	    webdavRemoteRoot: string;
	    pairingPort: number;
	    lanInstanceName: string;
	    s3Region: string;
	    s3Bucket: string;
	    s3Prefix: string;
	    s3AccessKey: string;
	    s3SecretKey: string;
	    s3Endpoint: string;
	
	    static createFrom(source: any = {}) {
	        return new SyncSettingsDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.webdavUrl = source["webdavUrl"];
	        this.webdavUser = source["webdavUser"];
	        this.webdavPassword = source["webdavPassword"];
	        this.webdavRemoteRoot = source["webdavRemoteRoot"];
	        this.pairingPort = source["pairingPort"];
	        this.lanInstanceName = source["lanInstanceName"];
	        this.s3Region = source["s3Region"];
	        this.s3Bucket = source["s3Bucket"];
	        this.s3Prefix = source["s3Prefix"];
	        this.s3AccessKey = source["s3AccessKey"];
	        this.s3SecretKey = source["s3SecretKey"];
	        this.s3Endpoint = source["s3Endpoint"];
	    }
	}
	export class LANPeerDTO {
	    name: string;
	    host: string;
	    ip: string;
	    port: number;
	    txt?: string[];
	
	    static createFrom(source: any = {}) {
	        return new LANPeerDTO(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.name = source["name"];
	        this.host = source["host"];
	        this.ip = source["ip"];
	        this.port = source["port"];
	        this.txt = source["txt"];
	    }
	}

}

export namespace domain {
	
	export class BlockMetadata {
	    sourcePath: string;
	    lineStart: number;
	    lineEnd: number;
	    level: number;
	    // Go type: time
	    createdAt: any;
	    // Go type: time
	    updatedAt: any;
	
	    static createFrom(source: any = {}) {
	        return new BlockMetadata(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.sourcePath = source["sourcePath"];
	        this.lineStart = source["lineStart"];
	        this.lineEnd = source["lineEnd"];
	        this.level = source["level"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
	        this.updatedAt = this.convertValues(source["updatedAt"], null);
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
	export class Block {
	    id: string;
	    parentId: string;
	    content: string;
	    properties?: Record<string, string>;
	    metadata: BlockMetadata;
	
	    static createFrom(source: any = {}) {
	        return new Block(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.parentId = source["parentId"];
	        this.content = source["content"];
	        this.properties = source["properties"];
	        this.metadata = this.convertValues(source["metadata"], BlockMetadata);
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

export namespace storage {
	
	export class BlockSearchHit {
	    id: string;
	    sourcePath: string;
	    content: string;
	    lineStart: number;
	    lineEnd: number;
	    outlineLevel: number;
	    snippet: string;
	    rank: number;
	
	    static createFrom(source: any = {}) {
	        return new BlockSearchHit(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.sourcePath = source["sourcePath"];
	        this.content = source["content"];
	        this.lineStart = source["lineStart"];
	        this.lineEnd = source["lineEnd"];
	        this.outlineLevel = source["outlineLevel"];
	        this.snippet = source["snippet"];
	        this.rank = source["rank"];
	    }
	}
	export class WikiGraphEdge {
	    source: string;
	    target: string;
	
	    static createFrom(source: any = {}) {
	        return new WikiGraphEdge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.target = source["target"];
	    }
	}
	export class WikiGraphNode {
	    id: string;
	    label: string;
	
	    static createFrom(source: any = {}) {
	        return new WikiGraphNode(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.label = source["label"];
	    }
	}
	export class WikiGraph {
	    nodes: WikiGraphNode[];
	    edges: WikiGraphEdge[];
	
	    static createFrom(source: any = {}) {
	        return new WikiGraph(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.nodes = this.convertValues(source["nodes"], WikiGraphNode);
	        this.edges = this.convertValues(source["edges"], WikiGraphEdge);
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
	
	
	export class WikiGraphSemanticEdge {
	    source: string;
	    target: string;
	    score: number;
	
	    static createFrom(source: any = {}) {
	        return new WikiGraphSemanticEdge(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.source = source["source"];
	        this.target = source["target"];
	        this.score = source["score"];
	    }
	}

}

