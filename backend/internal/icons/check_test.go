package icons

import (
	"bytes"
	"encoding/binary"
	"hash/crc32"
	"image"
	"image/color"
	"image/gif"
	"image/jpeg"
	"image/png"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.SetNRGBA(x, y, color.NRGBA{R: uint8(x), G: uint8(y), B: 200, A: 255})
		}
	}
	var b bytes.Buffer
	require.NoError(t, png.Encode(&b, img))
	return b.Bytes()
}

// hugePNGHeader is a PNG whose header claims 100000×100000 pixels: a small
// file that would expand into a huge image.
func hugePNGHeader() []byte {
	var b bytes.Buffer
	b.WriteString("\x89PNG\r\n\x1a\n")
	ihdr := make([]byte, 13)
	binary.BigEndian.PutUint32(ihdr[0:], 100000)
	binary.BigEndian.PutUint32(ihdr[4:], 100000)
	ihdr[8], ihdr[9] = 8, 6
	chunk := append([]byte("IHDR"), ihdr...)
	_ = binary.Write(&b, binary.BigEndian, uint32(len(ihdr)))
	b.Write(chunk)
	_ = binary.Write(&b, binary.BigEndian, crc32.ChecksumIEEE(chunk))
	return b.Bytes()
}

// webpOf is a 1×1 lossless WebP, written by hand: the standard library and
// x/image decode WebP but do not encode it.
var webpOf = []byte{
	'R', 'I', 'F', 'F', 0x1a, 0, 0, 0, 'W', 'E', 'B', 'P', 'V', 'P', '8', 'L',
	0x0d, 0, 0, 0, 0x2f, 0, 0, 0, 0x10, 0x07, 0x10, 0x11, 0x11, 0x88, 0x88, 0xfe, 0x07, 0, 0,
}

// A raster image is decoded under bounds and kept as a PNG of at most 256
// pixels a side; an SVG is kept as it is.
func TestCheckIcon(t *testing.T) {
	data, format, err := CheckIcon(pngOf(t, 600, 300))
	require.NoError(t, err)
	assert.Equal(t, FormatPNG, format)
	cfg, err := png.DecodeConfig(bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, [2]int{256, 128}, [2]int{cfg.Width, cfg.Height}, "a 600×300 PNG shrinks to 256×128")

	data, _, err = CheckIcon(pngOf(t, 100, 300))
	require.NoError(t, err)
	cfg, err = png.DecodeConfig(bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, [2]int{85, 256}, [2]int{cfg.Width, cfg.Height})

	var jb bytes.Buffer
	require.NoError(t, jpeg.Encode(&jb, image.NewRGBA(image.Rect(0, 0, 40, 40)), nil))
	_, format, err = CheckIcon(jb.Bytes())
	require.NoError(t, err, "JPEG")
	assert.Equal(t, FormatPNG, format)

	var gb bytes.Buffer
	require.NoError(t, gif.Encode(&gb, image.NewPaletted(image.Rect(0, 0, 16, 16), color.Palette{color.Black, color.White}), nil))
	_, format, err = CheckIcon(gb.Bytes())
	require.NoError(t, err, "GIF")
	assert.Equal(t, FormatPNG, format)

	data, format, err = CheckIcon(webpOf)
	require.NoError(t, err, "WebP")
	assert.Equal(t, FormatPNG, format)
	cfg, err = png.DecodeConfig(bytes.NewReader(data))
	require.NoError(t, err)
	assert.Equal(t, [2]int{1, 1}, [2]int{cfg.Width, cfg.Height})

	svg := `<?xml version="1.0"?><!-- c --><svg xmlns="http://www.w3.org/2000/svg" xmlns:xlink="http://www.w3.org/1999/xlink" viewBox="0 0 24 24">` +
		`<defs><linearGradient id="g"/></defs><style>.a{fill:url(#g)}</style>` +
		`<rect width="24" height="24" fill="url(#g)"/><use href="#g"/>` +
		`<image xlink:href="data:image/png;base64,iVBORw0KGgo=" width="1" height="1"/></svg>`
	data, format, err = CheckIcon([]byte(svg))
	require.NoError(t, err)
	assert.Equal(t, FormatSVG, format)
	assert.Equal(t, svg, string(data), "an SVG is kept byte for byte")
}

func TestCheckIconRefuses(t *testing.T) {
	big := pngOf(t, 600, 300)
	for name, raw := range map[string]string{
		"text":             "hello",
		"empty":            "",
		"bomb":             string(hugePNGHeader()),
		"truncated":        string(big[:100]),
		"a broken WebP":    "RIFF\x00\x00\x00\x00WEBPVP8 ",
		"doctype":          `<?xml version="1.0"?><!DOCTYPE svg [<!ENTITY x "y">]><svg>&x;</svg>`,
		"script":           `<svg><script>alert(1)</script></svg>`,
		"handler":          `<svg onload="alert(1)"/>`,
		"href":             `<svg><image href="file:///etc/passwd"/></svg>`,
		"xlink":            `<svg xmlns:xlink="http://www.w3.org/1999/xlink"><use xlink:href="https://example.com/x.svg#a"/></svg>`,
		"an embedded SVG":  `<svg><image href="data:image/svg+xml;base64,PHN2Zz4="/></svg>`,
		"css url":          `<svg><rect style="fill:url(https://example.com/x)"/></svg>`,
		"style url":        `<svg><style>rect{fill:url( 'http://x/y')}</style></svg>`,
		"style import":     `<svg><style>@import "https://example.com/x.css";</style></svg>`,
		"an imported attr": `<svg><rect style="@import 'x'"/></svg>`,
		"foreign":          `<svg><foreignObject><div/></foreignObject></svg>`,
		"animate":          `<svg><animate attributeName="href" to="javascript:alert(1)"/></svg>`,
		"not svg":          `<?xml version="1.0"?><html/>`,
		"two roots":        `<svg/><svg/>`,
		"pi":               `<svg><?php echo 1 ?></svg>`,
		"malformed":        `<svg><g></svg>`,
		"text after":       `<svg/>trailing`,
		"no root at all":   `<?xml version="1.0"?>`,
		"too large":        "<svg>" + strings.Repeat(" ", MaxSVG) + "</svg>",
		"too deep":         "<svg>" + strings.Repeat("<g>", 70) + strings.Repeat("</g>", 70) + "</svg>",
		"too many tokens":  "<svg>" + strings.Repeat("<g/>", 10001) + "</svg>",
	} {
		_, _, err := CheckIcon([]byte(raw))
		assert.Error(t, err, name)
	}
}

// An error names no content: what a remote host or a ConfigMap sent goes
// nowhere but through the check.
func TestCheckIconErrorsNameNoContent(t *testing.T) {
	_, _, err := CheckIcon([]byte(`<svg><image href="https://secret.example.com/token"/></svg>`))
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret")
}

func TestContentType(t *testing.T) {
	assert.Equal(t, "image/svg+xml", ContentType(FormatSVG))
	assert.Equal(t, "image/png", ContentType(FormatPNG))
}
