package icons

import (
	"bytes"
	"encoding/xml"
	"errors"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/gif"
	"image/jpeg"
	"image/png"
	"io"
	"strings"

	"golang.org/x/image/webp"
)

// The content check of an icon that does not come from a bundled catalogue,
// and of every file the pack builder puts into a pack (docs/adr/0009 D2):
// a port of Cogpit's CheckIcon (internal/engine/clustersettings_icon.go).
// A raster image is decoded under bounds — its header first, so a small
// file cannot expand into a huge image — and encoded again as a PNG of at
// most IconSide pixels a side: what the browser decodes is small and written
// by Dash. An SVG is kept as text after a check that it is one <svg>
// document without a DOCTYPE, scripts, event handlers or references outside
// itself.
//
// Where Dash differs from Cogpit: GIF and WebP are decoded too; an SVG may
// embed a PNG, JPEG, GIF or WebP as a data: URI, which many catalogue logos
// do and which runs nothing; a <style> may not @import.

// Bounds of a checked icon.
const (
	// MaxSVG is the most bytes of an SVG kept.
	MaxSVG = 256 << 10
	// IconSide is the most pixels a side of a raster icon kept.
	IconSide = 256

	maxIconPixels = 16 << 20 // width × height of a raster image decoded
	maxIconEdge   = 8192     // pixels a side of a raster image decoded
	maxSVGTokens  = 20000
	maxSVGDepth   = 64
)

// The formats of a checked icon, as its file's extension.
const (
	FormatPNG = "png"
	FormatSVG = "svg"
)

// ContentType is the media type of a checked icon's format.
func ContentType(format string) string {
	if format == FormatSVG {
		return "image/svg+xml"
	}
	return "image/png"
}

// CheckIcon checks an image's bytes and returns them as Dash serves them: a
// PNG, JPEG, GIF or WebP decoded under bounds and encoded again as a small
// PNG, an SVG checked and kept as it is. Its errors name no content.
func CheckIcon(raw []byte) (data []byte, format string, err error) {
	if decode, decodeConfig := rasterCodec(raw); decode != nil {
		data, err := rasterIcon(raw, decode, decodeConfig)
		return data, FormatPNG, err
	}
	if looksLikeSVG(raw) {
		if err := CheckSVG(raw); err != nil {
			return nil, "", err
		}
		return raw, FormatSVG, nil
	}
	return nil, "", errors.New("the icon is not a PNG, JPEG, GIF, WebP or SVG image")
}

type (
	decodeFunc       func(io.Reader) (image.Image, error)
	decodeConfigFunc func(io.Reader) (image.Config, error)
)

// rasterCodec picks the decoder by the format's magic bytes; nil for none.
func rasterCodec(raw []byte) (decodeFunc, decodeConfigFunc) {
	switch {
	case bytes.HasPrefix(raw, []byte("\x89PNG\r\n\x1a\n")):
		return png.Decode, png.DecodeConfig
	case bytes.HasPrefix(raw, []byte{0xff, 0xd8, 0xff}):
		return jpeg.Decode, jpeg.DecodeConfig
	case bytes.HasPrefix(raw, []byte("GIF87a")), bytes.HasPrefix(raw, []byte("GIF89a")):
		return gif.Decode, gif.DecodeConfig
	case len(raw) >= 12 && bytes.Equal(raw[:4], []byte("RIFF")) && bytes.Equal(raw[8:12], []byte("WEBP")):
		return webp.Decode, webp.DecodeConfig
	}
	return nil, nil
}

// rasterIcon decodes a raster image, its size checked from its header first,
// and encodes it again as a PNG of at most IconSide pixels a side.
func rasterIcon(raw []byte, decode decodeFunc, decodeConfig decodeConfigFunc) ([]byte, error) {
	cfg, err := decodeConfig(bytes.NewReader(raw))
	if err != nil {
		return nil, errors.New("the icon is not a readable image")
	}
	if cfg.Width < 1 || cfg.Height < 1 || cfg.Width > maxIconEdge || cfg.Height > maxIconEdge || cfg.Width*cfg.Height > maxIconPixels {
		return nil, fmt.Errorf("the icon is %d×%d pixels; Dash takes at most %d pixels a side and %d megapixels",
			cfg.Width, cfg.Height, maxIconEdge, maxIconPixels>>20)
	}
	img, err := decode(bytes.NewReader(raw))
	if err != nil {
		return nil, errors.New("the icon is not a readable image")
	}
	var out bytes.Buffer
	if err := png.Encode(&out, shrink(img, IconSide)); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

// shrink scales img down so that neither side exceeds side pixels,
// averaging the source pixels each target pixel covers; a smaller image is
// only copied.
func shrink(img image.Image, side int) *image.NRGBA {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= side && h <= side {
		out := image.NewNRGBA(image.Rect(0, 0, w, h))
		draw.Draw(out, out.Bounds(), img, b.Min, draw.Src)
		return out
	}
	tw, th := side, side
	if w > h {
		th = max(1, h*side/w)
	} else {
		tw = max(1, w*side/h)
	}
	out := image.NewNRGBA(image.Rect(0, 0, tw, th))
	for ty := range th {
		y0, y1 := b.Min.Y+ty*h/th, b.Min.Y+(ty+1)*h/th
		for tx := range tw {
			x0, x1 := b.Min.X+tx*w/tw, b.Min.X+(tx+1)*w/tw
			out.SetNRGBA(tx, ty, average(img, x0, max(x1, x0+1), y0, max(y1, y0+1)))
		}
	}
	return out
}

// average is the mean colour of img's pixels in [x0,x1)×[y0,y1).
func average(img image.Image, x0, x1, y0, y1 int) color.NRGBA {
	var r, g, b, a, n uint64
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			c := color.NRGBA64Model.Convert(img.At(x, y)).(color.NRGBA64)
			r, g, b, a, n = r+uint64(c.R), g+uint64(c.G), b+uint64(c.B), a+uint64(c.A), n+1
		}
	}
	// #nosec G115 -- the mean of 16-bit values shifted right by 8 is below 256.
	return color.NRGBA{R: uint8(r / n >> 8), G: uint8(g / n >> 8), B: uint8(b / n >> 8), A: uint8(a / n >> 8)}
}

// looksLikeSVG reports whether raw starts, after a byte order mark and white
// space, with an XML declaration, a comment or an <svg element.
func looksLikeSVG(raw []byte) bool {
	s := bytes.TrimLeft(bytes.TrimPrefix(raw, []byte("\xef\xbb\xbf")), " \t\r\n")
	return bytes.HasPrefix(s, []byte("<?xml")) || bytes.HasPrefix(s, []byte("<svg")) || bytes.HasPrefix(s, []byte("<!--"))
}

// svgForbidden are the elements an icon may not hold: scripts, embedded
// HTML, frames, and anything that loads or animates content.
var svgForbidden = map[string]bool{"script": true, "foreignobject": true, "iframe": true, "object": true, "embed": true,
	"audio": true, "video": true, "animate": true, "set": true, "animatemotion": true, "animatetransform": true, "handler": true}

// CheckSVG checks that raw is one <svg> document: well-formed XML of bounded
// size, depth and token count, no DOCTYPE or other directive (no entities),
// no processing instruction but the XML declaration, none of svgForbidden's
// elements, no event handler attribute (on…), no @import, and no reference
// to anything but a fragment of itself (href="#…", url(#…)) or an embedded
// raster image (data:image/png;base64,…).
func CheckSVG(raw []byte) error {
	if len(raw) > MaxSVG {
		return fmt.Errorf("an SVG icon has at most %d KiB", MaxSVG>>10)
	}
	c := svgChecker{dec: xml.NewDecoder(bytes.NewReader(raw))}
	c.dec.Strict = true
	for {
		tok, err := c.dec.Token() // checks that elements nest and close
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return errors.New("the SVG icon is not well-formed XML")
		}
		if err := c.token(tok); err != nil {
			return err
		}
	}
	if !c.root {
		return errors.New("the icon is not an <svg> document")
	}
	return nil
}

type svgChecker struct {
	dec           *xml.Decoder
	depth, tokens int
	root          bool
}

func (c *svgChecker) token(tok xml.Token) error {
	if c.tokens++; c.tokens > maxSVGTokens {
		return errors.New("the SVG icon is too complex")
	}
	switch t := tok.(type) {
	case xml.Directive:
		return errors.New("the SVG icon has a DOCTYPE or another directive; Dash takes none")
	case xml.ProcInst:
		if t.Target != "xml" {
			return fmt.Errorf("the SVG icon has a processing instruction <?%s?>", t.Target)
		}
	case xml.StartElement:
		return c.start(t)
	case xml.EndElement:
		c.depth--
	case xml.CharData:
		if c.depth == 0 && len(bytes.TrimSpace(t)) > 0 {
			return errors.New("the icon is not one <svg> document")
		}
		return checkStyleText(string(t))
	}
	return nil
}

func (c *svgChecker) start(t xml.StartElement) error {
	name := strings.ToLower(t.Name.Local)
	if c.depth == 0 {
		if c.root || name != "svg" {
			return errors.New("the icon is not one <svg> document")
		}
		c.root = true
	}
	if c.depth++; c.depth > maxSVGDepth {
		return errors.New("the SVG icon is nested too deeply")
	}
	if svgForbidden[name] {
		return fmt.Errorf("the SVG icon has a <%s> element; Dash takes none", t.Name.Local)
	}
	for _, a := range t.Attr {
		if err := checkSVGAttr(a); err != nil {
			return err
		}
	}
	return nil
}

func checkSVGAttr(a xml.Attr) error {
	name, v := strings.ToLower(a.Name.Local), strings.TrimSpace(a.Value)
	switch {
	case strings.HasPrefix(name, "on"):
		return fmt.Errorf("the SVG icon has an event handler (%s); Dash takes none", a.Name.Local)
	case name == "href" || name == "src":
		if v != "" && !strings.HasPrefix(v, "#") && !isRasterDataURI(v) {
			return errors.New("the SVG icon refers to something outside itself")
		}
	default:
		return checkStyleText(v)
	}
	return nil
}

// checkStyleText refuses a style that loads anything: an @import, or a
// url(…) that is no fragment of the document.
func checkStyleText(s string) error {
	low := strings.ToLower(s)
	if strings.Contains(low, "@import") {
		return errors.New("the SVG icon imports a style sheet")
	}
	if strings.Contains(low, "url(") && !onlyFragmentURLs(low) {
		return errors.New("the SVG icon refers to something outside itself")
	}
	return nil
}

// rasterDataPrefixes are the embedded images an SVG may carry: raster data
// runs nothing, an embedded SVG could.
var rasterDataPrefixes = []string{"data:image/png;base64,", "data:image/jpeg;base64,", "data:image/jpg;base64,",
	"data:image/gif;base64,", "data:image/webp;base64,"}

func isRasterDataURI(v string) bool {
	low := strings.ToLower(v)
	for _, p := range rasterDataPrefixes {
		if strings.HasPrefix(low, p) {
			return true
		}
	}
	return false
}

// onlyFragmentURLs reports whether every url(…) in s, in lower case, names a
// fragment of the document, url(#id).
func onlyFragmentURLs(low string) bool {
	for {
		i := strings.Index(low, "url(")
		if i < 0 {
			return true
		}
		rest := strings.TrimLeft(low[i+4:], " \t\r\n'\"")
		if !strings.HasPrefix(rest, "#") {
			return false
		}
		low = rest
	}
}
