package imagestyle

import (
	"bytes"
	"encoding/binary"
	"errors"
	"hash/crc32"
	"testing"

	"github.com/disintegration/imaging"
)

// pngHeader - a PNG's signature and header chunk claiming width × height,
// without any pixels.
func pngHeader(width, height uint32) []byte {
	var b bytes.Buffer
	b.WriteString("\x89PNG\r\n\x1a\n")
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], width)
	binary.BigEndian.PutUint32(ihdr[4:], height)
	ihdr[8], ihdr[9] = 8, 6 // 8 bits, RGBA
	chunk := append([]byte("IHDR"), ihdr...)
	_ = binary.Write(&b, binary.BigEndian, uint32(len(ihdr)))
	b.Write(chunk)
	_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return b.Bytes()
}

// TestProcessRefusesSources - a file claiming a huge size in its header
// is refused before it is decoded, and so is one that isn't an image.
func TestProcessRefusesSources(t *testing.T) {
	style, err := NewImageStyle("card", "Card", 10, 10, ResizeModeFill, imaging.Center, 80)
	if err != nil {
		t.Fatal(err)
	}
	for name, data := range map[string][]byte{
		"huge.png":  pngHeader(65535, 65535),
		"wide.png":  pngHeader(MaxSourceSide+1, 10),
		"doc.pdf":   []byte("%PDF-1.4\n%\xe2\xe3\xcf\xd3\n"),
		"empty.png": nil,
	} {
		if _, err := NewImagingProcessor().Process(bytes.NewReader(data), name, style, style.Anchor(), nil); !errors.Is(err, ErrNotRenderable) {
			t.Errorf("%s: %v, want ErrNotRenderable", name, err)
		}
	}
}
