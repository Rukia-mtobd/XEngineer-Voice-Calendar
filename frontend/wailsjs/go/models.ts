export namespace llm {
	
	export class DeleteIntent {
	    action: string;
	    id: number;
	    date: string;
	    title: string;
	
	    static createFrom(source: any = {}) {
	        return new DeleteIntent(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.action = source["action"];
	        this.id = source["id"];
	        this.date = source["date"];
	        this.title = source["title"];
	    }
	}
	export class ParsedSchedule {
	    title: string;
	    date: string;
	    startTime: string;
	    endTime: string;
	    duration: number;
	    desc: string;
	
	    static createFrom(source: any = {}) {
	        return new ParsedSchedule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.date = source["date"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.duration = source["duration"];
	        this.desc = source["desc"];
	    }
	}

}

export namespace storage {
	
	export class ScheduleRecord {
	    id: number;
	    date: string;
	    title: string;
	    startTime: string;
	    endTime: string;
	    duration: number;
	    desc: string;
	    // Go type: time
	    createdAt: any;
	
	    static createFrom(source: any = {}) {
	        return new ScheduleRecord(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.date = source["date"];
	        this.title = source["title"];
	        this.startTime = source["startTime"];
	        this.endTime = source["endTime"];
	        this.duration = source["duration"];
	        this.desc = source["desc"];
	        this.createdAt = this.convertValues(source["createdAt"], null);
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

