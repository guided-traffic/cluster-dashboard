// Package badges puts badges on the icons of apps and bookmarks, in the
// style of macOS dock badges (docs/adr/0011). This file is the vocabulary:
// the corners, the tones and the grammar of one entry of an item's own list —
// an element of the annotation dash.gtrfc.com/badges, or of the badges list
// of a custom app or a bookmark.
package badges

import (
	"errors"
	"regexp"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"
)

// Corner is where a badge sits on its icon.
type Corner string

// The corners; top-right is where macOS puts a badge.
const (
	TopRight    Corner = "top-right"
	TopLeft     Corner = "top-left"
	BottomRight Corner = "bottom-right"
	BottomLeft  Corner = "bottom-left"
)

// Corners are every corner, the default first.
var Corners = []Corner{TopRight, TopLeft, BottomRight, BottomLeft}

// DefaultCorner is the corner of a badge that names none.
const DefaultCorner = TopRight

// IsCorner reports whether s names a corner.
func IsCorner(s string) bool {
	return slices.Contains(Corners, Corner(s))
}

// Tone is a badge's colour: one of the palette's lights (docs/adr/0005).
// Violet is the agent's in Cogpit and is no tone.
type Tone string

// The tones; coral is macOS's red.
const (
	Neutral Tone = "neutral"
	Blue    Tone = "blue"
	Cyan    Tone = "cyan"
	Mint    Tone = "mint"
	Amber   Tone = "amber"
	Coral   Tone = "coral"
)

// Tones are every tone, the default first.
var Tones = []Tone{Neutral, Blue, Cyan, Mint, Amber, Coral}

// DefaultTone is the tone of a definition that names none.
const DefaultTone = Neutral

// IsTone reports whether s names a tone.
func IsTone(s string) bool {
	return slices.Contains(Tones, Tone(s))
}

// MaxText is the most characters a text badge holds: a pill on a 40-pixel
// icon has no room for more; a number is a text.
const MaxText = 8

// definitionName is the form of a definition's name.
var definitionName = regexp.MustCompile(`^[a-z0-9]([-a-z0-9]*[a-z0-9])?$`)

// IsDefinitionName reports whether s can name a definition: lower case
// letters, digits and '-', at most 63 characters, as a Kubernetes name.
func IsDefinitionName(s string) bool {
	return len(s) <= 63 && definitionName.MatchString(s)
}

// ValidText reports whether s can be a badge's text: one to MaxText
// characters, no control character, no ',' (which separates the
// annotation's entries) and no '@' (which names a corner).
func ValidText(s string) bool {
	n := utf8.RuneCountInString(s)
	if n == 0 || n > MaxText || !utf8.ValidString(s) {
		return false
	}
	return !strings.ContainsFunc(s, func(r rune) bool { return unicode.IsControl(r) || r == ',' || r == '@' })
}

// Entry is one badge an item names itself: a definition by name, or an
// ad-hoc text whose label is its text.
type Entry struct {
	// Name is the definition's name; empty for a text badge.
	Name string
	// Text is the ad-hoc badge's text; empty for a definition.
	Text string
	// Corner is the corner the entry asks for, DefaultCorner when it names
	// none.
	Corner Corner
}

// The errors of ParseEntry.
var (
	ErrEntryEmpty  = errors.New("is empty")
	ErrEntryCorner = errors.New("names no corner after '@': top-right, top-left, bottom-right or bottom-left")
	ErrEntryName   = errors.New("must name a badge definition: lower case letters, digits and '-'")
	ErrEntryText   = errors.New("must be text: followed by one to eight characters, without ',' or '@'")
)

// textPrefix starts an ad-hoc text badge.
const textPrefix = "text:"

// ParseEntry parses one entry: name, name@corner, text:<value> or
// text:<value>@corner. Space around the entry, its corner and its text is
// dropped; space inside a text is kept.
func ParseEntry(raw string) (Entry, error) {
	s := strings.TrimSpace(raw)
	if s == "" {
		return Entry{}, ErrEntryEmpty
	}
	e := Entry{Corner: DefaultCorner}
	if i := strings.LastIndexByte(s, '@'); i >= 0 {
		corner := strings.TrimSpace(s[i+1:])
		if !IsCorner(corner) {
			return Entry{}, ErrEntryCorner
		}
		e.Corner = Corner(corner)
		s = strings.TrimSpace(s[:i])
	}
	if text, ok := strings.CutPrefix(s, textPrefix); ok {
		text = strings.TrimSpace(text)
		if !ValidText(text) {
			return Entry{}, ErrEntryText
		}
		e.Text = text
		return e, nil
	}
	if !IsDefinitionName(s) {
		return Entry{}, ErrEntryName
	}
	e.Name = s
	return e, nil
}

// String is the entry as it is written: name@corner or text:<value>@corner.
func (e Entry) String() string {
	if e.Text != "" {
		return textPrefix + e.Text + "@" + string(e.Corner)
	}
	return e.Name + "@" + string(e.Corner)
}
