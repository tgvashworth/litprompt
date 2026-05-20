package config

import (
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"
)

// --- helpers ---

func writeFile(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}

func setupTree(t *testing.T, files map[string]string) string {
	t.Helper()
	dir := t.TempDir()
	for rel, content := range files {
		writeFile(t, filepath.Join(dir, rel), content)
	}
	return dir
}

func sortedSrcOut(rs []Resolved) []string {
	out := make([]string, 0, len(rs))
	for _, r := range rs {
		out = append(out, r.Source+" -> "+r.Output)
	}
	sort.Strings(out)
	return out
}

// --- Load ---

func TestLoad_returnsNilWhenNoConfig(t *testing.T) {
	cfg, err := Load(t.TempDir())
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg != nil {
		t.Errorf("expected nil config, got %#v", cfg)
	}
}

func TestLoad_findsYaml(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "litprompt.yaml"), "builds:\n  - source: a.md\n    output: b.md\n")
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil || len(cfg.Builds) != 1 {
		t.Fatalf("expected 1 build, got %#v", cfg)
	}
	if cfg.Builds[0].Source != "a.md" || cfg.Builds[0].Output != "b.md" {
		t.Errorf("unexpected build: %#v", cfg.Builds[0])
	}
}

func TestLoad_findsYml(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "litprompt.yml"), "builds:\n  - source: a.md\n    output: b.md\n")
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil || len(cfg.Builds) != 1 {
		t.Fatalf("expected 1 build, got %#v", cfg)
	}
}

func TestLoad_errorsOnMalformedYAML(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "litprompt.yaml"), "builds: [not closed\n")
	if _, err := Load(dir); err == nil {
		t.Error("expected error for malformed yaml, got nil")
	}
}

func TestLoad_errorsWhenBothYamlAndYml(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "litprompt.yaml"), "builds: []\n")
	writeFile(t, filepath.Join(dir, "litprompt.yml"), "builds: []\n")
	_, err := Load(dir)
	if err == nil {
		t.Fatal("expected error when both files present, got nil")
	}
	if !strings.Contains(err.Error(), "both") && !strings.Contains(err.Error(), "ambiguous") {
		t.Errorf("error should mention ambiguity, got: %v", err)
	}
}

func TestLoad_errorsOnEmptyBuilds(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, filepath.Join(dir, "litprompt.yaml"), "builds: []\n")
	if _, err := Load(dir); err == nil {
		t.Error("expected error when builds is empty, got nil")
	}
}

// --- Resolve: single file ---

func TestResolve_singleFile_pathOutput(t *testing.T) {
	dir := setupTree(t, map[string]string{"a.md": "x"})
	cfg := &Config{Builds: []BuildSpec{{Source: "a.md", Output: "out/b.md"}}}
	got, err := cfg.Resolve(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"a.md -> out/b.md"}
	if g := sortedSrcOut(got); !equal(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestResolve_singleFile_siblingOutput(t *testing.T) {
	dir := setupTree(t, map[string]string{"foo/a.src.md": "x"})
	cfg := &Config{Builds: []BuildSpec{{Source: "foo/a.src.md", Output: "a.md"}}}
	got, err := cfg.Resolve(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"foo/a.src.md -> foo/a.md"}
	if g := sortedSrcOut(got); !equal(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestResolve_singleFile_directoryOutput(t *testing.T) {
	dir := setupTree(t, map[string]string{"a.md": "x"})
	cfg := &Config{Builds: []BuildSpec{{Source: "a.md", Output: "out/"}}}
	got, err := cfg.Resolve(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"a.md -> out/a.md"}
	if g := sortedSrcOut(got); !equal(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestResolve_singleFile_missing_errors(t *testing.T) {
	cfg := &Config{Builds: []BuildSpec{{Source: "nope.md", Output: "x.md"}}}
	if _, err := cfg.Resolve(t.TempDir()); err == nil {
		t.Error("expected error for missing source file, got nil")
	}
}

// --- Resolve: directory ---

func TestResolve_directoryMode_mirrorsTree(t *testing.T) {
	dir := setupTree(t, map[string]string{
		"src/a.md":         "x",
		"src/sub/b.md":     "y",
		"src/skip.txt":     "skipped",
	})
	cfg := &Config{Builds: []BuildSpec{{Source: "src/", Output: "out/"}}}
	got, err := cfg.Resolve(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		"src/a.md -> out/a.md",
		"src/sub/b.md -> out/sub/b.md",
	}
	if g := sortedSrcOut(got); !equal(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestResolve_directoryMode_bareFilenameOutput(t *testing.T) {
	dir := setupTree(t, map[string]string{"src/a.md": "x"})
	cfg := &Config{Builds: []BuildSpec{{Source: "src/", Output: "out"}}}
	got, err := cfg.Resolve(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{"src/a.md -> out/a.md"}
	if g := sortedSrcOut(got); !equal(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestResolve_directoryMode_emptyDir_errors(t *testing.T) {
	dir := setupTree(t, map[string]string{"src/skip.txt": "x"})
	cfg := &Config{Builds: []BuildSpec{{Source: "src/", Output: "out/"}}}
	if _, err := cfg.Resolve(dir); err == nil {
		t.Error("expected error when directory has no .md files, got nil")
	}
}

// --- Resolve: glob ---

func TestResolve_glob_siblingOutput(t *testing.T) {
	dir := setupTree(t, map[string]string{
		"plugins/data/skills/query/SKILL.src.md": "x",
		"plugins/data/skills/chart/SKILL.src.md": "y",
	})
	cfg := &Config{Builds: []BuildSpec{
		{Source: "plugins/*/skills/*/SKILL.src.md", Output: "SKILL.md"},
	}}
	got, err := cfg.Resolve(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		"plugins/data/skills/chart/SKILL.src.md -> plugins/data/skills/chart/SKILL.md",
		"plugins/data/skills/query/SKILL.src.md -> plugins/data/skills/query/SKILL.md",
	}
	if g := sortedSrcOut(got); !equal(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestResolve_glob_doublestarSiblingOutput(t *testing.T) {
	dir := setupTree(t, map[string]string{
		"a/b/c.src.md":   "x",
		"a/b/d/e.src.md": "y",
	})
	cfg := &Config{Builds: []BuildSpec{
		{Source: "a/**/*.src.md", Output: "out.md"},
	}}
	got, err := cfg.Resolve(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := []string{
		"a/b/c.src.md -> a/b/out.md",
		"a/b/d/e.src.md -> a/b/d/out.md",
	}
	if g := sortedSrcOut(got); !equal(g, want) {
		t.Errorf("got %v, want %v", g, want)
	}
}

func TestResolve_glob_pathOutput_errors(t *testing.T) {
	dir := setupTree(t, map[string]string{"src/a.md": "x"})
	cfg := &Config{Builds: []BuildSpec{{Source: "src/*.md", Output: "out/foo.md"}}}
	_, err := cfg.Resolve(dir)
	if err == nil {
		t.Fatal("expected error for glob source with path output, got nil")
	}
	if !strings.Contains(err.Error(), "sibling") && !strings.Contains(err.Error(), "bare filename") {
		t.Errorf("error should explain sibling rule, got: %v", err)
	}
}

func TestResolve_glob_directoryOutput_errors(t *testing.T) {
	dir := setupTree(t, map[string]string{"src/a.md": "x"})
	cfg := &Config{Builds: []BuildSpec{{Source: "src/*.md", Output: "out/"}}}
	if _, err := cfg.Resolve(dir); err == nil {
		t.Error("expected error for glob source with directory output, got nil")
	}
}

func TestResolve_glob_noMatches_errors(t *testing.T) {
	dir := setupTree(t, map[string]string{"src/a.md": "x"})
	cfg := &Config{Builds: []BuildSpec{{Source: "src/*.src.md", Output: "out.md"}}}
	if _, err := cfg.Resolve(dir); err == nil {
		t.Error("expected error when glob matches nothing, got nil")
	}
}

// --- Resolve: per-build header ---

func TestResolve_carriesHeader(t *testing.T) {
	dir := setupTree(t, map[string]string{"a.md": "x"})
	cfg := &Config{Builds: []BuildSpec{{Source: "a.md", Output: "b.md", Header: "full"}}}
	got, err := cfg.Resolve(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Header != "full" {
		t.Errorf("expected header 'full', got %#v", got)
	}
}

func TestResolve_invalidHeader_errors(t *testing.T) {
	dir := setupTree(t, map[string]string{"a.md": "x"})
	cfg := &Config{Builds: []BuildSpec{{Source: "a.md", Output: "b.md", Header: "wrong"}}}
	if _, err := cfg.Resolve(dir); err == nil {
		t.Error("expected error for invalid header, got nil")
	}
}

// --- Resolve: shared validation ---

func TestResolve_emptySource_errors(t *testing.T) {
	cfg := &Config{Builds: []BuildSpec{{Source: "", Output: "b.md"}}}
	if _, err := cfg.Resolve(t.TempDir()); err == nil {
		t.Error("expected error for empty source, got nil")
	}
}

func TestResolve_emptyOutput_errors(t *testing.T) {
	dir := setupTree(t, map[string]string{"a.md": "x"})
	cfg := &Config{Builds: []BuildSpec{{Source: "a.md", Output: ""}}}
	if _, err := cfg.Resolve(dir); err == nil {
		t.Error("expected error for empty output, got nil")
	}
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
