-- Rollback: add replay gain columns back
ALTER TABLE tracks ADD COLUMN replay_gain_track REAL DEFAULT 0;
ALTER TABLE tracks ADD COLUMN replay_gain_album REAL DEFAULT 0;
