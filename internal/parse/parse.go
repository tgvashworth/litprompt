package parse

import (
	"regexp"
	"strings"
)

// Import represents an @[label](target) directive found in the source.
type Import struct {
	// Line is the zero-based line index where the import appears.
	Line int
	// Label is the text inside the square brackets.
	Label string
	// Target is the path or URL inside the parentheses.
	Target string
}

// IsRemote returns true if the import target is an HTTP(S) URL.
func (i Import) IsRemote() bool {
	return strings.HasPrefix(i.Target, "https://") || strings.HasPrefix(i.Target, "http://")
}

var (
	// commentPattern matches <!-- @ ... --> blocks (possibly multi-line),
	// including the trailing newline if present (the comment's own line ending).
	commentPattern = regexp.MustCompile(`(?s)<!-- @.*?-->\n?`)

	// importPattern matches @[label](target) at the start of a line,
	// with optional leading whitespace.
	importPattern = regexp.MustCompile(`(?m)^\s*@\[([^\]]+)\]\(([^)]+)\)$`)
)

// StripComments removes all <!-- @ ... --> comment blocks from content,
// then collapses any resulting runs of multiple blank lines to at most one,
// and trims leading/trailing blank lines.
func StripComments(content string) string {
	result := commentPattern.ReplaceAllString(content, "")

	// Collapse runs of 3+ newlines (2+ blank lines) down to 2 newlines (1 blank line)
	multiBlank := regexp.MustCompile(`\n{3,}`)
	result = multiBlank.ReplaceAllString(result, "\n\n")

	// Trim leading and trailing whitespace (blank lines)
	result = strings.TrimLeft(result, "\n")
	result = strings.TrimRight(result, "\n")

	// If there's any content, ensure exactly one trailing newline
	if len(result) > 0 {
		result += "\n"
	}

	return result
}

// FindImports returns all @[label](target) directives found in content.
// Only matches lines where the directive is at the start (with optional
// leading whitespace).
func FindImports(content string) []Import {
	var imports []Import

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		matches := importPattern.FindStringSubmatch(line)
		if matches != nil {
			imports = append(imports, Import{
				Line:   i,
				Label:  matches[1],
				Target: matches[2],
			})
		}
	}

	return imports
}
