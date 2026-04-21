package media

import "strings"

// SupportedAudioExts is a set of all audio file extensions that can be used.
// See https://en.wikipedia.org/wiki/HTML_audio#Supported_audio_coding_formats for common formats supported by browsers.
//
// TODO: support more formats, especially audiophile popular ones like ALAC, APE, WV, etc, but need to support transcoding
// with ffmpeg first
var supportedAudioExts = map[string]struct{}{
	".mp3":  {},
	".flac": {},
	".ogg":  {},
	".opus": {},
	".m4a":  {},
	".aac":  {},
	".wav":  {},
	".weba": {}, // basically webm, but its audio only.
	".aiff": {},
}

// losslessCodecs is a set of lossless or uncompressed audio codecs that should
// always be transcoded to a lossy format, as they typically exceed target bitrates.
var losslessCodecs = map[string]struct{}{
	"flac":    {},
	"pcm":     {},
	"aiff":    {},
	"wav":     {},
	"alac":    {},
	"ape":     {},
	"wavpack": {},
}

// IsLosslessCodec reports whether the given codec is a lossless or uncompressed format
// that should always be transcoded. The list of codecs can be found in the losslessCodecs map.
func IsLosslessCodec(codec string) bool {
	codec = strings.ToLower(codec)
	_, exists := losslessCodecs[codec]
	return exists
}

// contentTypes maps supported audio extensions to their standard MIME types.
// https://developer.mozilla.org/en-US/docs/Web/HTTP/Guides/MIME_types/Common_types
var contentTypes = map[string]string{
	".mp3":  "audio/mpeg",
	".flac": "audio/flac",
	".ogg":  "audio/ogg",
	".opus": "audio/opus",
	".m4a":  "audio/mp4",
	".aac":  "audio/mp4",
	".wav":  "audio/wav",
	".aiff": "audio/aiff",
	".weba": "audio/webm",
}

// MimeFromExt returns the MIME type for a given file extension, or binary data,
// "application/octet-stream" if unknown.
func MimeFromExt(ext string) string {
	ext = strings.ToLower(ext)
	if ct, exists := contentTypes[ext]; exists {
		return ct
	}
	return "application/octet-stream"
}

// IsSupportedAudio checks if the extension exists.
// Ensure it includes the dot, e.g., ".mp3".
func IsSupportedAudio(ext string) bool {
	_, exists := supportedAudioExts[strings.ToLower(ext)]
	return exists
}

// DesktopFilterPattern generates the "*.mp3;*.flac;..." string for native file dialogs in the desktop app.
func DesktopFilterPattern() string {
	var patterns []string
	for ext := range supportedAudioExts {
		patterns = append(patterns, "*"+ext)
	}
	return strings.Join(patterns, ";")
}
