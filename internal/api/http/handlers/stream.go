package handlers

import (
	"io"
	"time"
)

/*
Problem: if you have a 50 MB FLAC file and a fast
connection, Chromium browsers and other similar ones will try to download the entire 50 MB in about two seconds.
If the user listens to 10 seconds of one song and moves on to the next song, it became wasted bandwidth since
user didn't listen to the rest of the song that was already downloaded.

Goal here is to get Chromium and other browsers that greedily download the entire file to buffer in 1 continuous connection
by spoon feeding them at a constant rate by applying the token bucket algorithm. When the browser asks for more data than is available in the bucket, delay
until the bucket is refilled. This prevents the browser from buffering the entire file at once.

Read more:
https://en.wikipedia.org/wiki/Token_bucket

Important mechanics to know for this solution:
1. The fast burst: Start with a completely full bucket (e.g., 1.5MB of tokens). This allows the first 1.5MB
   to be sent over the wire instantly, ensuring the time to first audio is near 0 and makes playback feel instant
2. The seek reset: When the user scrubs/seeks through the song, a new Range request is made (calling Seek()).
   Immediately refill the bucket to the maximum to give the browser another fast burst at the new location,
   making scrubbing feel instantaneous. Without a reset, bucket would still be empty from previous playback and
   the user would have to wait for the bucket to refill, making scrubbing feel slow.

There are tradeoffs to this solution; since there is a long-lived TCP connection and goroutine open for each track; in theory,
this means if there are, let's say, 1,000 users consecutively streaming a different track, there will be 1,000 TCP
connections and 1,000 goroutines open. Though because target audience are self-hosters, this means way less users and thus
this is not really a problem. It's most likely a problem at large scale, of course.

An alternative to this solution is to just serve directly with http.ServeContent, which is simpler, but it leads to the problem originally faced;
browsers buffering the entire file at once, which can be problematic when dealing with .flac files and other high quality formats. Another
is to force users to transcode instead, but what if users want to listen to their high quality content *all* the time?
*/

// streamThrottleBytesPerSec is the maximum number of bytes per second the server
// will deliver for a single audio stream, preventing browsers from greedily buffering
// entire lossless files into memory while still allowing the browser to control its own HTTP byte range requests.
//
// 1.5 MB/s seems like a good threshold, not too big or small.
const streamThrottleBytesPerSec = 1536 * 1024 // 1.5 MB/s

// throttledReadSeeker wraps an io.ReadSeeker and rate-limits Read calls using a
// simple token-bucket approach so the browser cannot buffer the whole file at once.
type throttledReadSeeker struct {
	r          io.ReadSeeker
	rate       int64     // bytes per second
	bucket     int64     // currently available tokens
	lastRefill time.Time // last time the bucket was refilled
}

// newThrottledReadSeeker returns a new throttledReadSeeker.
func newThrottledReadSeeker(r io.ReadSeeker, bytesPerSec int64) *throttledReadSeeker {
	return &throttledReadSeeker{
		r:          r,
		rate:       bytesPerSec,
		bucket:     bytesPerSec, // start with a full bucket for instant Time to First Audio
		lastRefill: time.Now(),
	}
}

// refill adds tokens to the bucket based on elapsed time since the last refill.
func (t *throttledReadSeeker) refill() {
	now := time.Now()
	elapsed := now.Sub(t.lastRefill).Seconds()
	t.lastRefill = now
	gained := int64(elapsed * float64(t.rate))
	if gained > 0 {
		t.bucket += gained
		if t.bucket > t.rate {
			t.bucket = t.rate // cap at one second's worth of tokens
		}
	}
}

// Read attempts to read `p` bytes from the underlying io.ReadSeeker at the current position,
// but will block until at least one byte can be read or until 500ms have passed. This prevents
// the browser from buffering the entire file at once, saving bandwidth.
func (t *throttledReadSeeker) Read(p []byte) (int, error) {
	t.refill()

	if t.bucket <= 0 {
		// Sleep until we have at least one byte available
		sleepFor := min(time.Duration(float64(time.Second)*float64(-t.bucket+1)/float64(t.rate)), 500*time.Millisecond)
		time.Sleep(sleepFor)
		t.refill()
	}

	// Serve only up to what the bucket allows
	allowed := min(int64(len(p)), t.bucket)

	n, err := t.r.Read(p[:allowed])
	t.bucket -= int64(n)
	return n, err
}

// Seek resets bucket to allow a fast burst at the new position, giving the browser a quick response when the user scrubs.
func (t *throttledReadSeeker) Seek(offset int64, whence int) (int64, error) {
	t.bucket = t.rate
	t.lastRefill = time.Now()
	return t.r.Seek(offset, whence)
}
