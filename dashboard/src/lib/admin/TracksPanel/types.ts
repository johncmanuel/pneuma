export interface Track {
  id: string;
  title: string;
  album_artist: string;
  album_name: string;
  duration_ms: number;
  uploaded_by_user_id: string;
  created_at: string;
}

export type SortKey = "title" | "album_artist" | "album_name" | "duration_ms";
export type SortDir = "asc" | "desc";
