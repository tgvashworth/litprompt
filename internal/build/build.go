package build

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/tgvashworth/litprompt/internal/gitfetch"
	"github.com/tgvashworth/litprompt/internal/lockfile"
	"github.com/tgvashworth/litprompt/internal/parse"
)

// Options configures the build process.
type Options struct {
	// MockDir, if set, is used to resolve remote imports from a local
	// directory instead of fetching from the network.
	MockDir string

	// LockfilePath, if set, is used instead of discovering prompt.lock
	// next to the input file. The CLI sets this to <cwd>/prompt.lock.
	LockfilePath string

	// CacheDir is the directory for cached remote content (by hash).
	// Defaults to ~/.cache/litprompt/ if empty.
	CacheDir string
}

// importChain tracks the current import path for circular detection.
// It preserves insertion order so error messages show the full chain.
type importChain struct {
	paths []string
	set   map[string]bool
}

func newChain() *importChain {
	return &importChain{set: map[string]bool{}}
}

func (c *importChain) contains(path string) bool {
	return c.set[path]
}

func (c *importChain) push(path string) {
	c.paths = append(c.paths, path)
	c.set[path] = true
}

func (c *importChain) pop() {
	last := c.paths[len(c.paths)-1]
	c.paths = c.paths[:len(c.paths)-1]
	delete(c.set, last)
}

// circularError builds a message like "circular import detected: a.md -> b.md -> a.md"
func (c *importChain) circularError(target string) error {
	names := make([]string, len(c.paths))
	for i, p := range c.paths {
		names[i] = filepath.Base(p)
	}
	names = append(names, filepath.Base(target))
	return fmt.Errorf("circular import detected: %s", strings.Join(names, " -> "))
}

// Build processes a markdown file, stripping comments,
// resolving imports, and returning the final output.
func Build(inputPath string, opts Options) (string, error) {
	absPath, err := filepath.Abs(inputPath)
	if err != nil {
		return "", fmt.Errorf("resolving path: %w", err)
	}

	lockPath := opts.LockfilePath
	if lockPath == "" {
		lockPath = filepath.Join(filepath.Dir(absPath), "prompt.lock")
	}
	lf, _ := lockfile.Load(lockPath)

	chain := newChain()
	return buildFile(absPath, opts, lf, chain, true)
}

// BuildString processes markdown content from a string (e.g. stdin).
// baseDir is used to resolve relative imports.
func BuildString(content string, baseDir string, opts Options) (string, error) {
	absDir, err := filepath.Abs(baseDir)
	if err != nil {
		return "", fmt.Errorf("resolving base dir: %w", err)
	}

	lockPath := opts.LockfilePath
	if lockPath == "" {
		lockPath = filepath.Join(absDir, "prompt.lock")
	}
	lf, _ := lockfile.Load(lockPath)

	// Strip comments.
	result := parse.StripComments(content)

	// Resolve imports relative to baseDir.
	chain := newChain()
	stdinPath := filepath.Join(absDir, "<stdin>")
	chain.push(stdinPath)
	result, err = resolveImports(result, stdinPath, opts, lf, chain)
	if err != nil {
		return "", err
	}

	return result, nil
}

func buildFile(absPath string, opts Options, lf *lockfile.Lockfile, chain *importChain, isRoot bool) (string, error) {
	if chain.contains(absPath) {
		return "", chain.circularError(absPath)
	}
	chain.push(absPath)
	defer chain.pop()

	data, err := os.ReadFile(absPath)
	if err != nil {
		return "", fmt.Errorf("reading %s: %w", absPath, err)
	}

	content := string(data)

	if !isRoot {
		content = stripFrontmatter(content)
	}

	content = parse.StripComments(content)

	content, err = resolveImports(content, absPath, opts, lf, chain)
	if err != nil {
		return "", err
	}

	return content, nil
}

var importLinePattern = regexp.MustCompile(`^\s*@\[([^\]]+)\]\(([^)]+)\)$`)

func resolveImports(content string, fromPath string, opts Options, lf *lockfile.Lockfile, chain *importChain) (string, error) {
	fromDir := filepath.Dir(fromPath)
	lines := strings.Split(content, "\n")

	var result []string
	for _, line := range lines {
		matches := importLinePattern.FindStringSubmatch(line)
		if matches == nil {
			result = append(result, line)
			continue
		}

		target := matches[2]
		imported, err := resolveImport(target, fromDir, opts, lf, chain)
		if err != nil {
			return "", err
		}

		imported = strings.TrimRight(imported, "\n")
		result = append(result, imported)
	}

	return strings.Join(result, "\n"), nil
}

func resolveImport(target string, fromDir string, opts Options, lf *lockfile.Lockfile, chain *importChain) (string, error) {
	if strings.HasPrefix(target, "https://") || strings.HasPrefix(target, "http://") {
		return resolveRemoteImport(target, opts, lf, chain)
	}
	return resolveLocalImport(target, fromDir, opts, lf, chain)
}

func resolveLocalImport(target string, fromDir string, opts Options, lf *lockfile.Lockfile, chain *importChain) (string, error) {
	absTarget := filepath.Join(fromDir, target)
	absTarget, err := filepath.Abs(absTarget)
	if err != nil {
		return "", fmt.Errorf("resolving import path: %w", err)
	}

	if _, err := os.Stat(absTarget); os.IsNotExist(err) {
		return "", fmt.Errorf("import not found: %s", target)
	}

	return buildFile(absTarget, opts, lf, chain, false)
}

func resolveRemoteImport(url string, opts Options, lf *lockfile.Lockfile, chain *importChain) (string, error) {
	if lf == nil {
		return "", fmt.Errorf("no lockfile found for remote import: %s", url)
	}

	entry, ok := lf.Lookup(url)
	if !ok {
		return "", fmt.Errorf("remote import not in lockfile: %s", url)
	}

	content, err := fetchRemoteContent(url, opts, lf)
	if err != nil {
		return "", err
	}

	if err := entry.VerifyHash(content); err != nil {
		return "", fmt.Errorf("hash mismatch for %s", url)
	}

	result := stripFrontmatter(content)
	result = parse.StripComments(result)

	return result, nil
}

func fetchRemoteContent(url string, opts Options, lf *lockfile.Lockfile) (string, error) {
	// Test mode: read from mock directory.
	if opts.MockDir != "" {
		mockPath := urlToMockPath(url)
		fullPath := filepath.Join(opts.MockDir, mockPath)
		data, err := os.ReadFile(fullPath)
		if err != nil {
			return "", fmt.Errorf("reading mock content for %s: %w", url, err)
		}
		return string(data), nil
	}

	// Production mode: read from cache by content hash.
	if lf != nil {
		entry, ok := lf.Lookup(url)
		if ok {
			cacheDir := opts.CacheDir
			if cacheDir == "" {
				cacheDir = gitfetch.CacheDir()
			}
			hash := strings.TrimPrefix(entry.Hash, "sha256:")
			cachePath := filepath.Join(cacheDir, hash)
			if data, err := os.ReadFile(cachePath); err == nil {
				return string(data), nil
			}
		}
	}

	return "", fmt.Errorf("content not cached for %s (run 'litprompt lock' first)", url)
}

func urlToMockPath(rawURL string) string {
	path := rawURL
	path = strings.TrimPrefix(path, "https://")
	path = strings.TrimPrefix(path, "http://")
	path = strings.Replace(path, "/-/blob/", "/", 1)
	path = strings.Replace(path, "/blob/", "/", 1)
	return path
}

var frontmatterPattern = regexp.MustCompile(`(?s)\A---\n.*?\n---\n?`)

func stripFrontmatter(content string) string {
	loc := frontmatterPattern.FindStringIndex(content)
	if loc == nil {
		return content
	}
	result := content[loc[1]:]
	result = strings.TrimLeft(result, "\n")
	return result
}
