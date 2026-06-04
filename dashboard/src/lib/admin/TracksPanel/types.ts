export interface Track {
  id: string;
  title: string;
  artist_name?: string;
  album_artist: string;
  album_name: string;
  track_number: number;
  disc_number: number;
  duration_ms: number;
  uploaded_by_user_id?: string;
  created_at?: string;
  has_lyrics?: boolean;
}

export type SortKey = "title" | "album_artist" | "album_name" | "duration_ms";
export type SortDir = "asc" | "desc";
