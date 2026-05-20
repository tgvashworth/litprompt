// Package config loads and validates litprompt.yaml, the declarative build
// manifest. A Config expands into a list of Resolved (source, output) pairs.
package config

import (
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/bmatcuk/doublestar/v4"
	"go.yaml.in/yaml/v3"
)

// Config is the parsed litprompt.yaml.
type Config struct {
	Builds []BuildSpec `yaml:"builds"`
}

// BuildSpec is one entry in the builds list.
type BuildSpec struct {
	Source string `yaml:"source"`
	Output string `yaml:"output"`
	Header string `yaml:"header,omitempty"`
}

// Resolved is a single concrete build to run. Paths are relative to the
// directory passed to Resolve.
type Resolved struct {
	Source string
	Output string
	Header string
}

// Load reads litprompt.yaml or litprompt.yml from dir. Returns (nil, nil) if
// neither exists. Errors if both exist (ambiguous).
func Load(dir string) (*Config, error) {
	yamlPath := filepath.Join(dir, "litprompt.yaml")
	ymlPath := filepath.Join(dir, "litprompt.yml")
	yamlExists := fileExists(yamlPath)
	ymlExists := fileExists(ymlPath)

	if yamlExists && ymlExists {
		return nil, fmt.Errorf("ambiguous config: both litprompt.yaml and litprompt.yml exist in %s", dir)
	}

	var path string
	switch {
	case yamlExists:
		path = yamlPath
	case ymlExists:
		path = ymlPath
	default:
		return nil, nil
	}

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading %s: %w", path, err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing %s: %w", path, err)
	}
	if len(cfg.Builds) == 0 {
		return nil, fmt.Errorf("%s: no builds defined", path)
	}
	return &cfg, nil
}

// Resolve expands every BuildSpec into a list of concrete builds. dir is the
// base for relative source paths (typically the directory containing the
// config). Returns an error if any spec is invalid or matches no files.
func (c *Config) Resolve(dir string) ([]Resolved, error) {
	var out []Resolved
	for i, b := range c.Builds {
		items, err := resolveBuild(dir, b)
		if err != nil {
			return nil, fmt.Errorf("build[%d] (source=%q): %w", i, b.Source, err)
		}
		out = append(out, items...)
	}
	return out, nil
}

func resolveBuild(dir string, b BuildSpec) ([]Resolved, error) {
	if b.Source == "" {
		return nil, fmt.Errorf("source is required")
	}
	if b.Output == "" {
		return nil, fmt.Errorf("output is required")
	}
	if b.Header != "" && b.Header != "short" && b.Header != "full" {
		return nil, fmt.Errorf("invalid header %q: must be \"short\" or \"full\"", b.Header)
	}

	switch detectSourceShape(dir, b.Source) {
	case shapeFile:
		return resolveFile(dir, b)
	case shapeDir:
		return resolveDir(dir, b)
	case shapeGlob:
		return resolveGlob(dir, b)
	}
	return nil, fmt.Errorf("unknown source shape for %q", b.Source)
}

type sourceShape int

const (
	shapeFile sourceShape = iota
	shapeDir
	shapeGlob
)

func detectSourceShape(dir, src string) sourceShape {
	if strings.HasSuffix(src, "/") {
		return shapeDir
	}
	if hasGlobMeta(src) {
		return shapeGlob
	}
	if info, err := os.Stat(filepath.Join(dir, src)); err == nil && info.IsDir() {
		return shapeDir
	}
	return shapeFile
}

func hasGlobMeta(s string) bool {
	return strings.ContainsAny(s, "*?[{")
}

func isBareFilename(s string) bool {
	return !strings.Contains(s, "/")
}

func resolveFile(dir string, b BuildSpec) ([]Resolved, error) {
	if _, err := os.Stat(filepath.Join(dir, b.Source)); err != nil {
		return nil, fmt.Errorf("source not found: %s", b.Source)
	}
	out := b.Output
	if isBareFilename(out) {
		out = filepath.Join(filepath.Dir(b.Source), out)
	} else if strings.HasSuffix(out, "/") {
		out = filepath.Join(out, filepath.Base(b.Source))
	}
	return []Resolved{{Source: b.Source, Output: out, Header: b.Header}}, nil
}

func resolveDir(dir string, b BuildSpec) ([]Resolved, error) {
	srcDir := strings.TrimSuffix(b.Source, "/")
	outDir := strings.TrimSuffix(b.Output, "/")
	absSrc := filepath.Join(dir, srcDir)

	var matches []string
	err := filepath.WalkDir(absSrc, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !d.IsDir() && strings.HasSuffix(d.Name(), ".md") {
			rel, err := filepath.Rel(absSrc, path)
			if err != nil {
				return err
			}
			matches = append(matches, rel)
		}
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("walking %s: %w", srcDir, err)
	}
	if len(matches) == 0 {
		return nil, fmt.Errorf("no .md files found in %s", srcDir)
	}
	sort.Strings(matches)

	resolved := make([]Resolved, 0, len(matches))
	for _, rel := range matches {
		resolved = append(resolved, Resolved{
			Source: filepath.Join(srcDir, rel),
			Output: filepath.Join(outDir, rel),
			Header: b.Header,
		})
	}
	return resolved, nil
}

func resolveGlob(dir string, b BuildSpec) ([]Resolved, error) {
	if !isBareFilename(b.Output) {
		return nil, fmt.Errorf("glob source requires a sibling (bare filename) output, got %q", b.Output)
	}

	matches, err := doublestar.Glob(os.DirFS(dir), b.Source)
	if err != nil {
		return nil, fmt.Errorf("expanding glob %q: %w", b.Source, err)
	}

	// Filter out directories — only regular files are valid build sources.
	var files []string
	for _, m := range matches {
		info, err := os.Stat(filepath.Join(dir, m))
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", m, err)
		}
		if !info.IsDir() {
			files = append(files, m)
		}
	}
	if len(files) == 0 {
		return nil, fmt.Errorf("glob %q matched no files", b.Source)
	}
	sort.Strings(files)

	resolved := make([]Resolved, 0, len(files))
	for _, m := range files {
		resolved = append(resolved, Resolved{
			Source: m,
			Output: filepath.Join(filepath.Dir(m), b.Output),
			Header: b.Header,
		})
	}
	return resolved, nil
}

func fileExists(p string) bool {
	info, err := os.Stat(p)
	return err == nil && !info.IsDir()
}
