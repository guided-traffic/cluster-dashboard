package main

import (
	"bufio"
	"bytes"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/guided-traffic/dash/backend/internal/icons"
)

// checkRequired holds the built packs to required.txt: every reference must
// resolve and every "match <name>" find a logo, or the build fails and names
// each that does not.
func (b builder) checkRequired(path string) error {
	data, err := os.ReadFile(path) // #nosec G304 -- the pins' list
	if err != nil {
		return err
	}
	packs, err := icons.OpenPacks(b.out)
	if err != nil {
		return err
	}
	defer packs.Close()
	r, err := icons.NewResolver(icons.Options{
		Packs: packs, AutoMatch: []string{icons.CatalogSelfhst, icons.CatalogDashboardIcons},
		DefaultApp: "material-symbols:apps", DefaultBookmark: "material-symbols:bookmark",
	})
	if err != nil {
		return err
	}
	var missing []string
	checked := 0
	sc := bufio.NewScanner(bytes.NewReader(data))
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		checked++
		if name, ok := strings.CutPrefix(line, "match "); ok {
			if _, found := r.Match([]string{name}); !found {
				missing = append(missing, "auto-match finds no logo for "+name)
			}
			continue
		}
		if err := r.Register(line, false); err != nil {
			missing = append(missing, line+": "+err.Error())
		}
	}
	if len(missing) > 0 {
		return errors.New("the packs do not hold what required.txt names:\n  " + strings.Join(missing, "\n  "))
	}
	fmt.Fprintf(b.log, "required.txt: all %d resolve\n", checked)
	return nil
}
