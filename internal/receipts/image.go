package receipts

import (
	"bytes"
	"encoding/base64"
	"fmt"
	"strings"
)

// sniffImage validates the upload by content rather than trusting the browser's
// declared type, and returns the canonical MIME type to store.
func sniffImage(body []byte, declared string) (string, bool) {
	switch {
	case bytes.HasPrefix(body, []byte("\x89PNG\r\n\x1a\n")):
		return "image/png", true
	case bytes.HasPrefix(body, []byte("\xff\xd8\xff")):
		return "image/jpeg", true
	case bytes.HasPrefix(body, []byte("GIF87a")), bytes.HasPrefix(body, []byte("GIF89a")):
		return "image/gif", true
	case len(body) > 12 && bytes.Equal(body[0:4], []byte("RIFF")) && bytes.Equal(body[8:12], []byte("WEBP")):
		return "image/webp", true
	}
	head := strings.ToLower(strings.TrimSpace(string(body[:min(len(body), 512)])))
	if strings.Contains(head, "<svg") || strings.HasPrefix(head, "<?xml") && strings.Contains(head, "svg") {
		return "image/svg+xml", true
	}
	if strings.HasPrefix(strings.ToLower(declared), "image/") {
		return "", false
	}
	return "", false
}

func dataURI(contentType string, body []byte) string {
	if contentType == "" {
		contentType = "image/png"
	}
	return fmt.Sprintf("data:%s;base64,%s", contentType, base64.StdEncoding.EncodeToString(body))
}
