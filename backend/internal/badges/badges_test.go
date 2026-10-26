package badges

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseEntry(t *testing.T) {
	for raw, want := range map[string]Entry{
		"beta":                 {Name: "beta", Corner: TopRight},
		"beta@bottom-left":     {Name: "beta", Corner: BottomLeft},
		" beta @ top-left ":    {Name: "beta", Corner: TopLeft},
		"world-2@bottom-right": {Name: "world-2", Corner: BottomRight},
		"text:new":             {Text: "new", Corner: TopRight},
		"text:42@top-left":     {Text: "42", Corner: TopLeft},
		"text: v1.2 ":          {Text: "v1.2", Corner: TopRight},
		"text:in lab":          {Text: "in lab", Corner: TopRight},
		"text:äöü€":            {Text: "äöü€", Corner: TopRight},
		"text:12345678":        {Text: "12345678", Corner: TopRight},
	} {
		got, err := ParseEntry(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, want, got, raw)
	}
}

func TestParseEntryRefuses(t *testing.T) {
	for raw, want := range map[string]error{
		"":                 ErrEntryEmpty,
		"  ":               ErrEntryEmpty,
		"beta@middle":      ErrEntryCorner,
		"beta@":            ErrEntryCorner,
		"text:a@b":         ErrEntryCorner,
		"Beta":             ErrEntryName,
		"beta-":            ErrEntryName,
		"two words":        ErrEntryName,
		"@top-right":       ErrEntryName,
		"text:":            ErrEntryText,
		"text:123456789":   ErrEntryText,
		"text:a,b":         ErrEntryText,
		"text:\x07":        ErrEntryText,
		"text:  @top-left": ErrEntryText,
	} {
		_, err := ParseEntry(raw)
		assert.ErrorIs(t, err, want, raw)
	}
}

func TestEntryString(t *testing.T) {
	assert.Equal(t, "beta@top-right", Entry{Name: "beta", Corner: TopRight}.String())
	assert.Equal(t, "text:new@bottom-left", Entry{Text: "new", Corner: BottomLeft}.String())
}

func TestTheVocabulary(t *testing.T) {
	assert.Equal(t, TopRight, Corners[0], "the default corner comes first")
	assert.Equal(t, Neutral, Tones[0], "the default tone comes first")
	for _, c := range []string{"top-right", "top-left", "bottom-right", "bottom-left"} {
		assert.True(t, IsCorner(c), c)
	}
	assert.False(t, IsCorner("center"))
	for _, tone := range []string{"neutral", "blue", "cyan", "mint", "amber", "coral"} {
		assert.True(t, IsTone(tone), tone)
	}
	assert.False(t, IsTone("violet"), "violet is the agent's light in Cogpit")
	assert.True(t, IsDefinitionName("a"))
	assert.True(t, IsDefinitionName("world-2"))
	assert.False(t, IsDefinitionName(string(make([]byte, 64))))
	assert.False(t, IsDefinitionName("a_b"))
	assert.True(t, ValidText("€€€€€€€€"), "eight characters, not eight bytes")
	assert.False(t, ValidText(string([]byte{0xff})), "not UTF-8")
}
