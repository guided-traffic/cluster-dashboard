package main

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const service = `apiVersion: v1
kind: Service
metadata:
  name: dash-backend
`

func configMap(content string) string {
	return "apiVersion: v1\nkind: ConfigMap\nmetadata:\n  name: dash-config\ndata:\n  config.yaml: |\n" +
		indent(content) + "---\n"
}

func indent(s string) string {
	var b strings.Builder
	for line := range strings.Lines(s) {
		b.WriteString("    " + line)
	}
	return b.String()
}

func TestTheRenderedFileIsAccepted(t *testing.T) {
	var out strings.Builder
	err := run(strings.NewReader(service+"---\n"+configMap("ui:\n  title: Lab\n")), &out)
	require.NoError(t, err)
	assert.Equal(t, "dash-config/config.yaml: the backend accepts it\n", out.String())
}

func TestEveryProblemIsNamed(t *testing.T) {
	var out strings.Builder
	err := run(strings.NewReader(configMap("ui:\n  defaultTheme: sepia\n  refreshSeconds: 1\n")), &out)
	require.Error(t, err)
	assert.Equal(t, "the backend would refuse config.yaml of ConfigMap dash-config", err.Error())
	assert.Equal(t, "dash-config/config.yaml: line 2: ui.defaultTheme: must be system, light or dark\n"+
		"dash-config/config.yaml: line 3: ui.refreshSeconds: must be from 5 to 3600\n", out.String())
}

func TestNoConfigMapIsAFailure(t *testing.T) {
	err := run(strings.NewReader(service), &strings.Builder{})
	assert.EqualError(t, err, "no ConfigMap with config.yaml in the manifests")
}

func TestManifestsThatDoNotParseAreAFailure(t *testing.T) {
	err := run(strings.NewReader("kind: [\n"), &strings.Builder{})
	assert.ErrorContains(t, err, "read the manifests")
}
