export namespace db {
	
	export class Event {
	    id: number;
	    title: string;
	    scheduledAt: string;
	    reminded: boolean;
	    createdAt: string;
	
	    static createFrom(source: any = {}) {
	        return new Event(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.title = source["title"];
	        this.scheduledAt = source["scheduledAt"];
	        this.reminded = source["reminded"];
	        this.createdAt = source["createdAt"];
	    }
	}

}

