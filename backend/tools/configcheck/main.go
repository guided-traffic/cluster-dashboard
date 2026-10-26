// Command configcheck holds the configuration file the chart renders to the
// backend's own validation (docs/adr/0004 D4): it reads the manifests of
// `helm template` on its standard input, finds the ConfigMap that carries
// config.yaml, and parses that file as the backend does at start. It fails
// on every problem the backend would refuse to start with, and when no such
// ConfigMap is found.
//
// make helm-template runs it for every values file under ci/, so a value the
// schema admits and the backend refuses fails before a release does.
package main

import (
	"errors"
	"fmt"
	"io"
	"os"

	"go.yaml.in/yaml/v3"

	"github.com/guided-traffic/dash/backend/internal/config"
)

// fileKey is the ConfigMap key the backend's pods mount as DASH_CONFIG.
const fileKey = "config.yaml"

func main() {
	if err := run(os.Stdin, os.Stdout); err != nil {
		fmt.Fprintf(os.Stderr, "configcheck: %v\n", err)
		os.Exit(1)
	}
}

// manifest is what configcheck reads of each document.
type manifest struct {
	Kind     string `yaml:"kind"`
	Metadata struct {
		Name string `yaml:"name"`
	} `yaml:"metadata"`
	Data map[string]string `yaml:"data"`
}

func run(in io.Reader, out io.Writer) error {
	dec := yaml.NewDecoder(in)
	found := 0
	for {
		var m manifest
		err := dec.Decode(&m)
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return fmt.Errorf("read the manifests: %w", err)
		}
		content, ok := m.Data[fileKey]
		if m.Kind != "ConfigMap" || !ok {
			continue
		}
		found++
		f := config.DefaultFile()
		if problems := config.Parse([]byte(content), &f); len(problems) > 0 {
			for _, p := range problems {
				fmt.Fprintf(out, "%s/%s: %s\n", m.Metadata.Name, fileKey, p)
			}
			return fmt.Errorf("the backend would refuse %s of ConfigMap %s", fileKey, m.Metadata.Name)
		}
		fmt.Fprintf(out, "%s/%s: the backend accepts it\n", m.Metadata.Name, fileKey)
	}
	if found == 0 {
		return fmt.Errorf("no ConfigMap with %s in the manifests", fileKey)
	}
	return nil
}
