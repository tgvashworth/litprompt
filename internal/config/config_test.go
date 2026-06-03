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

func TestLoadFile_readsExplicitPath(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "litprompt.prod.yaml")
	writeFile(t, path, "builds:\n  - source: a.md\n    output: b.md\n")
	cfg, err := LoadFile(path)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg == nil || len(cfg.Builds) != 1 || cfg.Builds[0].Source != "a.md" {
		t.Fatalf("unexpected config: %#v", cfg)
	}
}

func TestLoadFile_missingFile_errors(t *testing.T) {
	_, err := LoadFile(filepath.Join(t.TempDir(), "nope.yaml"))
	if err == nil {
		t.Fatal("expected error for missing config file, got nil")
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
		"src/a.md":     "x",
		"src/sub/b.md": "y",
		"src/skip.txt": "skipped",
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

// --- Resolve: per-build interlock ---

func TestResolve_carriesInterlock(t *testing.T) {
	dir := setupTree(t, map[string]string{
		"plugins/a/skills/x/SKILL.src.md": "x",
		"plugins/a/skills/y/SKILL.src.md": "y",
	})
	cfg := &Config{Builds: []BuildSpec{{
		Source:    "plugins/*/skills/*/SKILL.src.md",
		Output:    "SKILL.md",
		Interlock: "enforce",
	}}}
	got, err := cfg.Resolve(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 resolved builds, got %d", len(got))
	}
	for _, r := range got {
		if r.Interlock != "enforce" {
			t.Errorf("expected interlock 'enforce' on %s, got %q", r.Source, r.Interlock)
		}
	}
}

func TestResolve_defaultsInterlockToOff(t *testing.T) {
	dir := setupTree(t, map[string]string{"a.md": "x"})
	cfg := &Config{Builds: []BuildSpec{{Source: "a.md", Output: "b.md"}}}
	got, err := cfg.Resolve(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Interlock != "off" {
		t.Errorf("expected interlock 'off', got %#v", got)
	}
}

func TestResolve_invalidInterlock_errors(t *testing.T) {
	dir := setupTree(t, map[string]string{"a.md": "x"})
	cfg := &Config{Builds: []BuildSpec{{Source: "a.md", Output: "b.md", Interlock: "wrong"}}}
	_, err := cfg.Resolve(dir)
	if err == nil {
		t.Fatal("expected error for invalid interlock, got nil")
	}
	if !strings.Contains(err.Error(), "analytics") && !strings.Contains(err.Error(), "enforce") {
		t.Errorf("error should list valid modes, got: %v", err)
	}
}

func TestLoad_parsesInterlockBlock(t *testing.T) {
	dir := setupTree(t, map[string]string{
		"a.md": "x",
		"litprompt.yaml": `interlock:
  param: tokens
  manifest: out/locks.json
  message:
    enforce: "must pass {token} as {param}"
builds:
  - source: a.md
    output: b.md
    interlock: analytics
`,
	})
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Interlock == nil {
		t.Fatal("expected interlock block to be parsed")
	}
	if cfg.Interlock.Param != "tokens" || cfg.Interlock.Manifest != "out/locks.json" {
		t.Errorf("unexpected interlock config: %#v", cfg.Interlock)
	}
	if cfg.Interlock.Message["enforce"] != "must pass {token} as {param}" {
		t.Errorf("unexpected message: %#v", cfg.Interlock.Message)
	}
	if cfg.Builds[0].Interlock != "analytics" {
		t.Errorf("expected per-build interlock 'analytics', got %q", cfg.Builds[0].Interlock)
	}
}

func TestResolve_carriesInterlockDirMode(t *testing.T) {
	dir := setupTree(t, map[string]string{
		"src/a.md":     "x",
		"src/sub/b.md": "y",
	})
	cfg := &Config{Builds: []BuildSpec{{Source: "src/", Output: "out/", Interlock: "analytics"}}}
	got, err := cfg.Resolve(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 2 {
		t.Fatalf("expected 2 resolved builds, got %d", len(got))
	}
	for _, r := range got {
		if r.Interlock != "analytics" {
			t.Errorf("expected interlock 'analytics' on %s, got %q", r.Source, r.Interlock)
		}
	}
}

func TestResolve_acceptsAllInterlockModes(t *testing.T) {
	cases := []struct{ in, want string }{
		{"", "off"},
		{"off", "off"},
		{"analytics", "analytics"},
		{"enforce", "enforce"},
	}
	for _, c := range cases {
		dir := setupTree(t, map[string]string{"a.md": "x"})
		cfg := &Config{Builds: []BuildSpec{{Source: "a.md", Output: "b.md", Interlock: c.in}}}
		got, err := cfg.Resolve(dir)
		if err != nil {
			t.Fatalf("mode %q: unexpected error: %v", c.in, err)
		}
		if len(got) != 1 || got[0].Interlock != c.want {
			t.Errorf("mode %q: expected resolved %q, got %#v", c.in, c.want, got)
		}
	}
}

func TestResolve_mixedInterlockModesPerBuild(t *testing.T) {
	dir := setupTree(t, map[string]string{
		"a.md": "x",
		"b.md": "y",
		"c.md": "z",
	})
	cfg := &Config{Builds: []BuildSpec{
		{Source: "a.md", Output: "out/a.md", Interlock: "analytics"},
		{Source: "b.md", Output: "out/b.md", Interlock: "enforce"},
		{Source: "c.md", Output: "out/c.md"},
	}}
	got, err := cfg.Resolve(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	want := map[string]string{"out/a.md": "analytics", "out/b.md": "enforce", "out/c.md": "off"}
	for _, r := range got {
		if want[r.Output] != r.Interlock {
			t.Errorf("output %s: expected interlock %q, got %q", r.Output, want[r.Output], r.Interlock)
		}
	}
}

func TestResolve_carriesHeaderAndInterlockTogether(t *testing.T) {
	dir := setupTree(t, map[string]string{"a.md": "x"})
	cfg := &Config{Builds: []BuildSpec{{Source: "a.md", Output: "b.md", Header: "full", Interlock: "enforce"}}}
	got, err := cfg.Resolve(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].Header != "full" || got[0].Interlock != "enforce" {
		t.Errorf("expected header 'full' and interlock 'enforce', got %#v", got)
	}
}

func TestInterlockSettings_appliesDefaults(t *testing.T) {
	cfg := &Config{}
	ic := cfg.InterlockSettings()
	if ic.Param != DefaultInterlockParam || ic.Manifest != DefaultInterlockManifest {
		t.Errorf("expected defaults, got %#v", ic)
	}

	cfg = &Config{Interlock: &InterlockConfig{Param: "custom"}}
	ic = cfg.InterlockSettings()
	if ic.Param != "custom" || ic.Manifest != DefaultInterlockManifest {
		t.Errorf("expected custom param with default manifest, got %#v", ic)
	}

	cfg = &Config{Interlock: &InterlockConfig{Manifest: "locks.json"}}
	ic = cfg.InterlockSettings()
	if ic.Param != DefaultInterlockParam || ic.Manifest != "locks.json" {
		t.Errorf("expected default param with custom manifest, got %#v", ic)
	}
}

func TestLoad_noInterlockBlock_perBuildStillCarried(t *testing.T) {
	dir := setupTree(t, map[string]string{
		"a.md":           "x",
		"litprompt.yaml": "builds:\n  - source: a.md\n    output: b.md\n    interlock: enforce\n",
	})
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Interlock != nil {
		t.Errorf("expected no top-level interlock block, got %#v", cfg.Interlock)
	}
	if cfg.Builds[0].Interlock != "enforce" {
		t.Errorf("expected per-build interlock 'enforce', got %q", cfg.Builds[0].Interlock)
	}
	// Defaults still apply when the block is omitted.
	if ic := cfg.InterlockSettings(); ic.Param != DefaultInterlockParam || ic.Manifest != DefaultInterlockManifest {
		t.Errorf("expected default settings, got %#v", ic)
	}
}

func TestLoad_interlockBlock_messageBothModes(t *testing.T) {
	dir := setupTree(t, map[string]string{
		"a.md": "x",
		"litprompt.yaml": `interlock:
  message:
    analytics: "log {token} via {param}"
    enforce: "require {token} via {param}"
builds:
  - source: a.md
    output: b.md
    interlock: analytics
`,
	})
	cfg, err := Load(dir)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if cfg.Interlock == nil {
		t.Fatal("expected interlock block to be parsed")
	}
	if cfg.Interlock.Message["analytics"] != "log {token} via {param}" {
		t.Errorf("unexpected analytics message: %#v", cfg.Interlock.Message)
	}
	if cfg.Interlock.Message["enforce"] != "require {token} via {param}" {
		t.Errorf("unexpected enforce message: %#v", cfg.Interlock.Message)
	}
	// param/manifest unset in the block fall back to defaults.
	if ic := cfg.InterlockSettings(); ic.Param != DefaultInterlockParam || ic.Manifest != DefaultInterlockManifest {
		t.Errorf("expected default param/manifest, got %#v", ic)
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
