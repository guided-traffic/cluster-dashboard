// Package config loads the backend's configuration: five environment
// variables — the file's path, the folders of the icon packs and of the icon
// ConfigMaps, the listen address and the log level — and one YAML file for
// everything else (docs/adr/0004). Load reports every invalid
// variable and every invalid key at once, so a broken configuration is fixed
// in one round, not one key per restart.
//
// The user-facing reference with the defaults is the configuration section
// of README.md; this package is what that section is checked against.
package config

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"slices"
	"strconv"
	"strings"
)

// The environment variables the backend reads.
const (
	EnvConfig         = "DASH_CONFIG"
	EnvIconPacks      = "DASH_ICON_PACKS"
	EnvIconConfigMaps = "DASH_ICON_CONFIGMAPS"
	EnvListenAddr     = "DASH_LISTEN_ADDR"
	EnvLogLevel       = "DASH_LOG_LEVEL"
)

// The defaults.
const (
	DefaultConfigPath = "/etc/dash/config.yaml"
	// DefaultIconPacks is where the backend image carries its icon packs
	// (backend/Containerfile).
	DefaultIconPacks = "/usr/share/dash/icons"
	// DefaultIconConfigMaps is where the chart mounts the icon ConfigMaps,
	// one folder each (deploy/helm/dash/templates/backend-deployment.yaml).
	DefaultIconConfigMaps = "/etc/dash-icons"
	DefaultListenAddr     = ":8080"
	DefaultLogLevel       = "info"
)

// logLevels are the accepted values of DASH_LOG_LEVEL.
var logLevels = map[string]slog.Level{
	"debug": slog.LevelDebug,
	"info":  slog.LevelInfo,
	"warn":  slog.LevelWarn,
	"error": slog.LevelError,
}

// Config is the complete configuration.
type Config struct {
	// ConfigPath is the file that was read; empty when none was: the
	// variable unset and nothing at DefaultConfigPath.
	ConfigPath string
	// IconPacks is the folder of the icon packs.
	IconPacks string
	// IconConfigMaps is the folder of the mounted icon ConfigMaps.
	IconConfigMaps string
	// ListenAddr is the address the HTTP server binds, host:port.
	ListenAddr string
	// LogLevel is the lowest level the log writes.
	LogLevel slog.Level
	// File is what the YAML file holds.
	File File
}

// Error is an invalid configuration: every problem Load found, each naming
// the variable or the file, the line and the key. A value is never echoed.
type Error struct {
	Problems []string
}

func (e *Error) Error() string {
	return "invalid configuration:\n  " + strings.Join(e.Problems, "\n  ")
}

// Load reads the configuration through lookup (os.LookupEnv) and readFile
// (os.ReadFile). It returns an *Error that lists every problem at once.
func Load(lookup func(string) (string, bool), readFile func(string) ([]byte, error)) (Config, error) {
	cfg := Config{
		IconPacks: DefaultIconPacks, IconConfigMaps: DefaultIconConfigMaps,
		ListenAddr: DefaultListenAddr, LogLevel: slog.LevelInfo, File: DefaultFile(),
	}
	var problems []string

	problems = append(problems, folders(lookup, map[string]*string{EnvIconPacks: &cfg.IconPacks, EnvIconConfigMaps: &cfg.IconConfigMaps})...)
	if v, ok := lookup(EnvListenAddr); ok {
		if err := validateListenAddr(v); err != nil {
			problems = append(problems, fmt.Sprintf("%s: %v", EnvListenAddr, err))
		} else {
			cfg.ListenAddr = v
		}
	}
	if v, ok := lookup(EnvLogLevel); ok {
		if level, known := logLevels[v]; known {
			cfg.LogLevel = level
		} else {
			problems = append(problems, fmt.Sprintf("%s: must be one of debug, info, warn, error", EnvLogLevel))
		}
	}

	path, explicit := lookup(EnvConfig)
	switch {
	case explicit && path == "":
		problems = append(problems, fmt.Sprintf("%s: must name a file when it is set", EnvConfig))
	default:
		if !explicit {
			path = DefaultConfigPath
		}
		data, err := readFile(path)
		switch {
		case err == nil:
			cfg.ConfigPath = path
			for _, p := range Parse(data, &cfg.File) {
				problems = append(problems, path+": "+p)
			}
		case !explicit && errors.Is(err, fs.ErrNotExist):
			// No file where none was asked for: the defaults apply.
		default:
			problems = append(problems, fmt.Sprintf("%s: cannot read %s: %v", EnvConfig, path, unwrapPath(err)))
		}
	}

	if len(problems) > 0 {
		return Config{}, &Error{Problems: problems}
	}
	return cfg, nil
}

// folders reads the variables that name a folder into their fields; an
// empty one is a problem.
func folders(lookup func(string) (string, bool), vars map[string]*string) []string {
	var problems []string
	for name, into := range vars {
		if v, ok := lookup(name); ok {
			if v == "" {
				problems = append(problems, fmt.Sprintf("%s: must name a folder when it is set", name))
			}
			*into = v
		}
	}
	slices.Sort(problems)
	return problems
}

// Parse decodes the YAML file's content into f, which holds the defaults the
// file may override, and validates the result. It returns every problem, in
// the order of their lines; none means f is ready to use.
func Parse(data []byte, f *File) []string {
	d := decodeStrict(data, f)
	found := append(d.problems, validate(f, d)...)
	if len(found) == 0 {
		return nil
	}
	slices.SortStableFunc(found, func(a, b problem) int { return a.line - b.line })
	out := make([]string, 0, len(found))
	for _, p := range found {
		out = append(out, p.String())
	}
	return out
}

// validateListenAddr accepts host:port with an empty host or a host name or
// address, and a port from 0 to 65535 (0 picks a free port).
func validateListenAddr(addr string) error {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return errors.New("must be host:port, such as :8080 or 127.0.0.1:8080")
	}
	if strings.ContainsAny(host, " \t") {
		return errors.New("the host must not contain spaces")
	}
	n, err := strconv.Atoi(port)
	if err != nil || n < 0 || n > 65535 {
		return errors.New("the port must be a number from 0 to 65535")
	}
	return nil
}

// unwrapPath drops the path an *fs.PathError repeats, since the problem
// names the file already.
func unwrapPath(err error) error {
	var pathErr *fs.PathError
	if errors.As(err, &pathErr) {
		return pathErr.Err
	}
	return err
}
