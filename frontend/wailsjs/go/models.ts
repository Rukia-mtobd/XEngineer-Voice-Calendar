export namespace llm {
	
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

