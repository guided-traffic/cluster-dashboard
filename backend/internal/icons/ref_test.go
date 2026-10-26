package icons

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestParseRef(t *testing.T) {
	for raw, want := range map[string]Ref{
		// A bare name is mdi's: Hajimari's rule, which the owner's manifests use.
		"web":             {Kind: RefCatalog, Catalog: CatalogMDI, Name: "web"},
		"home-automation": {Kind: RefCatalog, Catalog: CatalogMDI, Name: "home-automation"},
		"  pail  ":        {Kind: RefCatalog, Catalog: CatalogMDI, Name: "pail"},
		"Web":             {Kind: RefCatalog, Catalog: CatalogMDI, Name: "web"},
		// Every prefix and its long name.
		"mdi:web":                       {Kind: RefCatalog, Catalog: CatalogMDI, Name: "web"},
		"material-symbols:apps":         {Kind: RefCatalog, Catalog: CatalogMaterialSymbols, Name: "apps"},
		"material-symbols:home-outline": {Kind: RefCatalog, Catalog: CatalogMaterialSymbols, Name: "home-outline"},
		"si:grafana":                    {Kind: RefCatalog, Catalog: CatalogSimpleIcons, Name: "grafana"},
		"simple-icons:kubernetes":       {Kind: RefCatalog, Catalog: CatalogSimpleIcons, Name: "kubernetes"},
		"sh:grafana":                    {Kind: RefCatalog, Catalog: CatalogSelfhst, Name: "grafana"},
		"selfhst:home-assistant":        {Kind: RefCatalog, Catalog: CatalogSelfhst, Name: "home-assistant"},
		"di:alertmanager":               {Kind: RefCatalog, Catalog: CatalogDashboardIcons, Name: "alertmanager"},
		"dashboard-icons:1password":     {Kind: RefCatalog, Catalog: CatalogDashboardIcons, Name: "1password"},
		"SI:Grafana":                    {Kind: RefCatalog, Catalog: CatalogSimpleIcons, Name: "grafana"},
		"di:wiki.js":                    {Kind: RefCatalog, Catalog: CatalogDashboardIcons, Name: "wiki.js"},
		// ConfigMap keys keep their case: Kubernetes keys are case sensitive.
		"configmap:billing.jpg":     {Kind: RefConfigMap, Key: "billing.jpg"},
		"configmap:Team_Logo-2.PNG": {Kind: RefConfigMap, Key: "Team_Logo-2.PNG"},
		// A URL is anything with //.
		"https://example.com/logo.png":          {Kind: RefRemote, URL: "https://example.com/logo.png"},
		"http://icons.home.arpa:8080/a.svg?x=1": {Kind: RefRemote, URL: "http://icons.home.arpa:8080/a.svg?x=1"},
	} {
		got, err := ParseRef(raw)
		require.NoError(t, err, raw)
		assert.Equal(t, want, got, raw)
	}
}

func TestParseRefRefuses(t *testing.T) {
	for raw, want := range map[string]error{
		"":                              ErrRefEmpty,
		"   ":                           ErrRefEmpty,
		"ftp://example.com/a.png":       ErrRefURL,
		"//example.com/a.png":           ErrRefURL,
		"javascript://x":                ErrRefURL,
		"https://user:pw@example.com/a": ErrRefURL,
		"https:///a.png":                ErrRefURL,
		"configmap:":                    ErrRefConfigMapKey,
		"configmap:a/b":                 ErrRefConfigMapKey,
		"configmap:..":                  ErrRefConfigMapKey,
		"fa:home":                       ErrRefUnknownPrefix,
		"hajimari.io:web":               ErrRefUnknownPrefix,
		"mdi:":                          ErrRefName,
		"mdi:two words":                 ErrRefName,
		"mdi:a--b":                      ErrRefName,
		"-web":                          ErrRefName,
		"si:a:b":                        ErrRefName,
	} {
		_, err := ParseRef(raw)
		assert.ErrorIs(t, err, want, raw)
	}
}

// An error never echoes the reference: it can be a URL with a token.
func TestParseRefErrorsNameNoValue(t *testing.T) {
	_, err := ParseRef("ftp://example.com/secret-token")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "secret-token")
}

func TestRefString(t *testing.T) {
	for raw, want := range map[string]string{
		"web":                          "mdi:web",
		"selfhst:Grafana":              "sh:grafana",
		"configmap:a.png":              "configmap:a.png",
		"https://example.com/logo.png": "https://example.com/logo.png",
	} {
		ref, err := ParseRef(raw)
		require.NoError(t, err)
		assert.Equal(t, want, ref.String(), raw)
	}
	assert.Empty(t, Ref{}.String())
}

func TestGlyphCatalogs(t *testing.T) {
	for _, c := range []string{CatalogMDI, CatalogMaterialSymbols, CatalogSimpleIcons} {
		assert.True(t, IsGlyphCatalog(c), c)
	}
	for _, c := range []string{CatalogSelfhst, CatalogDashboardIcons, "", "fa"} {
		assert.False(t, IsGlyphCatalog(c), c)
	}
}
