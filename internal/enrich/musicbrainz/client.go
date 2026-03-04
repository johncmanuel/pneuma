package musicbrainz

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"pneuma/internal/models"
)

const (
	baseURL   = "https://musicbrainz.org/ws/2"
	userAgent = "pneuma/0.1 (https://github.com/pneuma-player/pneuma)"
)

// Client queries the MusicBrainz API at most once per second.
type Client struct {
	httpClient *http.Client
	mu         sync.Mutex
	lastCall   time.Time
}

// New creates a MusicBrainz client.
func New() *Client {
	return &Client{
		httpClient: &http.Client{Timeout: 10 * time.Second},
	}
}

func (c *Client) rateLimit() {
	c.mu.Lock()
	defer c.mu.Unlock()
	since := time.Since(c.lastCall)
	if since < time.Second {
		time.Sleep(time.Second - since)
	}
	c.lastCall = time.Now()
}

// EnrichTrack queries MusicBrainz for a recording matching the track.
func (c *Client) EnrichTrack(ctx context.Context, track *models.Track) error {
	if track.Title == "" {
		return nil
	}
	query := escapeLucene(track.Title)
	if track.AlbumArtist != "" {
		query += fmt.Sprintf(" AND artist:%s", escapeLucene(track.AlbumArtist))
	}

	c.rateLimit()

	u := fmt.Sprintf("%s/recording/?query=%s&limit=1&fmt=json",
		baseURL, url.QueryEscape(query))

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("musicbrainz request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("musicbrainz %d: %s", resp.StatusCode, string(body))
	}

	var result struct {
		Recordings []struct {
			ID       string `json:"id"`
			Title    string `json:"title"`
			Releases []struct {
				ID    string `json:"id"`
				Title string `json:"title"`
				Date  string `json:"date"`
			} `json:"releases"`
		} `json:"recordings"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("decode musicbrainz response: %w", err)
	}

	if len(result.Recordings) == 0 {
		return nil
	}
	rec := result.Recordings[0]
	track.MBRecordingID = rec.ID
	return nil
}

// LookupRelease fetches details for a MusicBrainz release.
func (c *Client) LookupRelease(ctx context.Context, mbid string) (*ReleaseInfo, error) {
	c.rateLimit()

	u := fmt.Sprintf("%s/release/%s?inc=artist-credits&fmt=json", baseURL, mbid)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, u, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("musicbrainz release %d", resp.StatusCode)
	}

	var info ReleaseInfo
	if err := json.NewDecoder(resp.Body).Decode(&info); err != nil {
		return nil, err
	}
	return &info, nil
}

// ReleaseInfo holds decoded release data.
type ReleaseInfo struct {
	ID    string `json:"id"`
	Title string `json:"title"`
	Date  string `json:"date"`
}

// escapeLucene escapes special characters in a Lucene query string.
func escapeLucene(s string) string {
	specials := []string{"+", "-", "&&", "||", "(", ")", "{", "}", "[", "]", "^", "~", "*", "?", ":", `"`}
	for _, sp := range specials {
		s = strings.ReplaceAll(s, sp, `\`+sp)
	}
	return s
}
