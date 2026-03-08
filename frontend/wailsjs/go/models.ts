export namespace desktop {
	
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
	export class LocalAlbumGroup {
	    key: string;
	    name: string;
	    artist: string;
	    track_count: number;
	    first_track_path: string;
	
	    static createFrom(source: any = {}) {
	        return new LocalAlbumGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.name = source["name"];
	        this.artist = source["artist"];
	        this.track_count = source["track_count"];
	        this.first_track_path = source["first_track_path"];
	    }
	}
	export class LocalAlbumGroupsResult {
	    albums: LocalAlbumGroup[];
	    total: number;
	
	    static createFrom(source: any = {}) {
	        return new LocalAlbumGroupsResult(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.albums = this.convertValues(source["albums"], LocalAlbumGroup);
	        this.total = source["total"];
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
	export class LocalDuplicateGroup {
	    key: string;
	    tracks: LocalTrack[];
	
	    static createFrom(source: any = {}) {
	        return new LocalDuplicateGroup(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.key = source["key"];
	        this.tracks = this.convertValues(source["tracks"], LocalTrack);
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

export namespace options {
	
	export class SecondInstanceData {
	    Args: string[];
	    WorkingDirectory: string;
	
	    static createFrom(source: any = {}) {
	        return new SecondInstanceData(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Args = source["Args"];
	        this.WorkingDirectory = source["WorkingDirectory"];
	    }
	}

}

