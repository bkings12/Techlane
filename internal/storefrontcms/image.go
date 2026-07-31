package storefrontcms

import "bytes"

// sniffImage validates the upload by content rather than trusting the
// browser's declared type, and returns the canonical MIME type to store.
func sniffImage(body []byte, declared string) (string, bool) {
	switch {
	case bytes.HasPrefix(body, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", true
	case bytes.HasPrefix(body, []byte("\xff\xd8\xff")):
		return "image/jpeg", true
	case len(body) > 12 && bytes.Equal(body[0:4], []byte("RIFF")) && bytes.Equal(body[8:12], []byte("WEBP")):
		return "image/webp", true
	}
	_ = declared
	return "", false
}
