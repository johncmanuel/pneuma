export namespace main {
	
	export class ConnectResult {
	    user: number[];
	    token: string;
	
	    static createFrom(source: any = {}) {
	        return new ConnectResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.user = source["user"];
	        this.token = source["token"];
	    }
	}
	export class LocalTrack {
	    path: string;
	    title: string;
	    artist: string;
	    album: string;
	    album_artist: string;
	    genre: string;
	    year: number;
	    track_number: number;
	    disc_number: number;
	    duration_ms: number;
	    has_artwork: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LocalTrack(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.path = source["path"];
	        this.title = source["title"];
	        this.artist = source["artist"];
	        this.album = source["album"];
	        this.album_artist = source["album_artist"];
	        this.genre = source["genre"];
	        this.year = source["year"];
	        this.track_number = source["track_number"];
	        this.disc_number = source["disc_number"];
	        this.duration_ms = source["duration_ms"];
	        this.has_artwork = source["has_artwork"];
	    }
	}

}

