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
	export class LocalPlaylistItem {
	    position: number;
	    source: string;
	    track_id?: string;
	    local_path?: string;
	    ref_title: string;
	    ref_album: string;
	    ref_album_artist: string;
	    ref_duration_ms: number;
	    added_at: string;
	    resolved: boolean;
	    missing: boolean;
	
	    static createFrom(source: any = {}) {
	        return new LocalPlaylistItem(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.position = source["position"];
	        this.source = source["source"];
	        this.track_id = source["track_id"];
	        this.local_path = source["local_path"];
	        this.ref_title = source["ref_title"];
	        this.ref_album = source["ref_album"];
	        this.ref_album_artist = source["ref_album_artist"];
	        this.ref_duration_ms = source["ref_duration_ms"];
	        this.added_at = source["added_at"];
	        this.resolved = source["resolved"];
	        this.missing = source["missing"];
	    }
	}
	export class LocalPlaylistSummary {
	    id: string;
	    name: string;
	    description: string;
	    artwork_path: string;
	    remote_playlist_id: string;
	    item_count: number;
	    total_duration_ms: number;
	    created_at: string;
	    updated_at: string;
	
	    static createFrom(source: any = {}) {
	        return new LocalPlaylistSummary(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.id = source["id"];
	        this.name = source["name"];
	        this.description = source["description"];
	        this.artwork_path = source["artwork_path"];
	        this.remote_playlist_id = source["remote_playlist_id"];
	        this.item_count = source["item_count"];
	        this.total_duration_ms = source["total_duration_ms"];
	        this.created_at = source["created_at"];
	        this.updated_at = source["updated_at"];
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
	export class RecentAlbum {
	    Key: string;
	    Name: string;
	    Artist: string;
	    IsLocal: boolean;
	    FirstTrackID: string;
	    FirstLocalPath: string;
	    PlayedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new RecentAlbum(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.Key = source["Key"];
	        this.Name = source["Name"];
	        this.Artist = source["Artist"];
	        this.IsLocal = source["IsLocal"];
	        this.FirstTrackID = source["FirstTrackID"];
	        this.FirstLocalPath = source["FirstLocalPath"];
	        this.PlayedAt = source["PlayedAt"];
	    }
	}
	export class RecentPlaylist {
	    ID: string;
	    Name: string;
	    ArtworkPath: string;
	    PlayedAt: number;
	
	    static createFrom(source: any = {}) {
	        return new RecentPlaylist(source);
	    }
	
	    constructor(source: any = {}) {
	        if ('string' === typeof source) source = JSON.parse(source);
	        this.ID = source["ID"];
	        this.Name = source["Name"];
	        this.ArtworkPath = source["ArtworkPath"];
	        this.PlayedAt = source["PlayedAt"];
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

