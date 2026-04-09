package lockfile

import (
	"crypto/sha256"
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Lockfile represents a parsed prompt.lock file.
type Lockfile struct {
	Imports map[string]Entry
}

// Entry represents a single import entry in the lockfile.
type Entry struct {
	Hash string // e.g. "sha256:abc123..."
}

// Lookup returns the entry for a URL, if it exists.
func (l *Lockfile) Lookup(url string) (Entry, bool) {
	e, ok := l.Imports[url]
	return e, ok
}

// VerifyHash checks that the given content matches this entry's hash.
func (e Entry) VerifyHash(content string) error {
	expected := e.Hash
	actual := "sha256:" + sha256Hex(content)
	if actual != expected {
		return fmt.Errorf("hash mismatch: expected %s, got %s", expected, actual)
	}
	return nil
}

func sha256Hex(content string) string {
	h := sha256.Sum256([]byte(content))
	return fmt.Sprintf("%x", h)
}

// Load reads and parses a prompt.lock file.
// Returns nil, nil if the file does not exist.
func Load(path string) (*Lockfile, error) {
	data, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("reading lockfile: %w", err)
	}

	return Parse(string(data))
}

// Parse parses lockfile content.
// Simple YAML-like parser — just enough for our format.
func Parse(content string) (*Lockfile, error) {
	lf := &Lockfile{Imports: map[string]Entry{}}

	urlPattern := regexp.MustCompile(`^\s+"([^"]+)":\s*$`)
	hashPattern := regexp.MustCompile(`^\s+hash:\s+"([^"]+)"\s*$`)

	lines := strings.Split(content, "\n")
	var currentURL string

	for _, line := range lines {
		if matches := urlPattern.FindStringSubmatch(line); matches != nil {
			currentURL = matches[1]
		} else if matches := hashPattern.FindStringSubmatch(line); matches != nil {
			if currentURL != "" {
				lf.Imports[currentURL] = Entry{Hash: matches[1]}
				currentURL = ""
			}
		}
	}

	return lf, nil
}
