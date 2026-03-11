-- Add columns introduced with the playlist feature that may be missing from
-- databases created before the full schema was in place.
ALTER TABLE playlists ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE playlists ADD COLUMN artwork_path TEXT NOT NULL DEFAULT '';
