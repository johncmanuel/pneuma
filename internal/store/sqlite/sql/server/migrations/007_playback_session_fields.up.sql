ALTER TABLE playback_sessions ADD COLUMN queue_index INTEGER DEFAULT 0;
ALTER TABLE playback_sessions ADD COLUMN repeat_mode INTEGER DEFAULT 0;
ALTER TABLE playback_sessions ADD COLUMN shuffle INTEGER DEFAULT 0;
ALTER TABLE playback_sessions ADD COLUMN playing INTEGER DEFAULT 0;
