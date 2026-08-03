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

// BuildESCPOSQR encodes content as a GS v 0 bit-image QR.
// Raster is used instead of native GS ( k because many POS-80 USB clones
// ignore the native QR commands entirely.
func BuildESCPOSQR(content string) []byte {
	content = strings.TrimSpace(content)
	if content == "" {
		return nil
	}
	return escposRasterQR(content, 5)
}

// escposRasterQR draws a QR as a GS v 0 bit image.
func escposRasterQR(content string, scale int) []byte {
	if scale < 2 {
		scale = 2
	}
	if scale > 8 {
		scale = 8
	}
	code, err := qrcode.New(content, qrcode.Medium)
	if err != nil {
		return nil
	}
	bm := code.Bitmap() // includes quiet zone
	modules := len(bm)
	if modules == 0 {
		return nil
	}
	widthDots := modules * scale
	heightDots := modules * scale
	widthBytes := (widthDots + 7) / 8
	row := make([]byte, widthBytes)
	var img []byte
	for y := 0; y < modules; y++ {
		for sy := 0; sy < scale; sy++ {
			for i := range row {
				row[i] = 0
			}
			for x := 0; x < modules; x++ {
				if !bm[y][x] {
					continue
				}
				for sx := 0; sx < scale; sx++ {
					dot := x*scale + sx
					row[dot/8] |= 0x80 >> uint(dot%8)
				}
			}
			img = append(img, row...)
		}
	}
	xL := byte(widthBytes & 0xff)
	xH := byte((widthBytes >> 8) & 0xff)
	yL := byte(heightDots & 0xff)
	yH := byte((heightDots >> 8) & 0xff)
	out := []byte{0x1d, 0x76, 0x30, 0x00, xL, xH, yL, yH}
	out = append(out, img...)
	out = append(out, '\n')
	return out
}
