package varsfile

import (
	"fmt"
	"os"
	"regexp"
	"strings"
)

// Load reads and merges multiple .env-format files in the order given.
// Later files override earlier ones on key collision.
func Load(paths []string) (map[string]string, error) {
	merged := map[string]string{}
	for _, path := range paths {
		data, err := os.ReadFile(path)
		if err != nil {
			return nil, fmt.Errorf("reading vars file %s: %w", path, err)
		}
		parsed, err := Parse(string(data))
		if err != nil {
			return nil, fmt.Errorf("parsing %s: %w", path, err)
		}
		for k, v := range parsed {
			merged[k] = v
		}
	}
	return merged, nil
}

// linePattern captures KEY and the raw rest-of-line after "=".
// Leading whitespace before KEY is allowed.
var linePattern = regexp.MustCompile(`^\s*([A-Za-z_][A-Za-z0-9_]*)=(.*)$`)

// Parse parses .env-format content into a map.
// Supports KEY=value, KEY="value", KEY='value', # comments, blank lines, CRLF.
// Trailing # comments are stripped from unquoted values. Duplicate keys: last wins.
func Parse(content string) (map[string]string, error) {
	out := map[string]string{}

	// Normalise CRLF.
	content = strings.ReplaceAll(content, "\r\n", "\n")
	content = strings.ReplaceAll(content, "\r", "")

	for _, line := range strings.Split(content, "\n") {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}
		m := linePattern.FindStringSubmatch(line)
		if m == nil {
			continue
		}
		key := m[1]
		out[key] = parseValue(m[2])
	}

	return out, nil
}

// parseValue interprets the substring after "=" on a single line.
func parseValue(raw string) string {
	v := strings.TrimLeft(raw, " \t")
	if v == "" {
		return ""
	}
	if v[0] == '"' || v[0] == '\'' {
		quote := v[0]
		// Find the matching closing quote.
		if end := strings.IndexByte(v[1:], quote); end >= 0 {
			return v[1 : 1+end]
		}
		// Unterminated quote: take everything after the opening quote.
		return v[1:]
	}
	// Unquoted: strip trailing # comment, then trim trailing whitespace.
	if hash := strings.Index(v, " #"); hash >= 0 {
		v = v[:hash]
	} else if hash := strings.Index(v, "\t#"); hash >= 0 {
		v = v[:hash]
	}
	return strings.TrimRight(v, " \t")
}
