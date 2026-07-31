package receipts

import (
	"encoding/base64"
	"fmt"
	"strings"

	qrcode "github.com/skip2/go-qrcode"
)

// QRDataURIPNG encodes content as a compact PNG data URI for thermal receipts.
func QRDataURIPNG(content string) (string, error) {
	content = strings.TrimSpace(content)
	if content == "" {
		return "", fmt.Errorf("empty qr payload")
	}
	png, err := qrcode.Encode(content, qrcode.Medium, 164)
	if err != nil {
		return "", err
	}
	return "data:image/png;base64," + base64.StdEncoding.EncodeToString(png), nil
}
