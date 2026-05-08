package desktop

import "testing"

func TestDedupeRandomTracks(t *testing.T) {
	candidates := []randomTrack{
		{source: "local_ref", title: "Song", album: "Album", albumArtist: "Artist", durationMS: 1000},
		{source: "remote", title: "song", album: "album", albumArtist: "artist", durationMS: 2000},
		{source: "local_ref", title: "Other", album: "Album", albumArtist: "Artist", durationMS: 3000},
	}

	deduped := dedupeRandomTracks(candidates)
	if len(deduped) != 2 {
		t.Fatalf("expected 2 deduped tracks, got %d", len(deduped))
	}

	if deduped[0].durationMS != 1000 {
		t.Fatalf("expected first duplicate to win, got duration %d", deduped[0].durationMS)
	}

	if deduped[1].title != "Other" {
		t.Fatalf("expected unique track to remain, got %q", deduped[1].title)
	}
}
