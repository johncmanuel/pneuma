-- SQLite 3.35+ supports DROP COLUMN; the columns revert to not existing.
ALTER TABLE playlists DROP COLUMN description;
ALTER TABLE playlists DROP COLUMN artwork_path;
