export namespace llm {
	
	export class ParsedSchedule {
	    title: string;
	    date: string;
	    time: string;
	    desc: string;
	
	    static createFrom(source: any = {}) {
	        return new ParsedSchedule(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.title = source["title"];
	        this.date = source["date"];
	        this.time = source["time"];
	        this.desc = source["desc"];
	    }
	}

}

