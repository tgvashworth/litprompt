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

	// importLinePattern matches @[label](target) at the start of a line,
	// with optional leading whitespace.
	importLinePattern = regexp.MustCompile(`^\s*@\[([^\]]+)\]\(([^)]+)\)$`)

	// varDirectivePattern matches a variable directive [{{placeholder}}](#NAME)
	// anywhere on a line. Used to keep inline directives from being flagged as
	// suspected imports. Variables are otherwise handled by the substitute pass,
	// not here — they carry no leading @ and so never match importLinePattern.
	varDirectivePattern = regexp.MustCompile(`\[\{\{([^}]*)\}\}\]\(#([A-Z_][A-Z0-9_]*)\)`)
)

// MatchImportLine returns the label and target if line is a valid import line.
// Returns ok=false if the line doesn't match the @[label](target) shape. The
// new variable syntax ([{{...}}](#NAME)) has no leading @, so it can never be
// mistaken for an import — no special-casing is needed here.
func MatchImportLine(line string) (label, target string, ok bool) {
	m := importLinePattern.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	return m[1], m[2], true
}

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
		label, target, ok := MatchImportLine(line)
		if !ok {
			continue
		}
		imports = append(imports, Import{
			Line:   i,
			Label:  label,
			Target: target,
		})
	}

	return imports
}

// suspectedImportPattern matches lines that start with @[...](...) but have
// trailing content — likely a typo where the user meant it to be an import.
var suspectedImportPattern = regexp.MustCompile(`(?m)^\s*@\[([^\]]+)\]\(([^)]+)\).+$`)

// SuspectedImport represents a line that looks like an import but isn't valid.
type SuspectedImport struct {
	Line    int
	Content string
}

// FindSuspectedImports returns lines that look like imports but have trailing
// content (e.g. `@[label](path).`). These are likely typos.
// Lines that contain a valid variable directive (e.g. `text [{{U}}](#NAME) more`)
// are not flagged — those are inline variable substitutions, not malformed imports.
func FindSuspectedImports(content string) []SuspectedImport {
	var suspected []SuspectedImport
	validImports := map[int]bool{}
	for _, imp := range FindImports(content) {
		validImports[imp.Line] = true
	}

	lines := strings.Split(content, "\n")
	for i, line := range lines {
		if validImports[i] {
			continue
		}
		if varDirectivePattern.MatchString(line) {
			continue
		}
		if suspectedImportPattern.MatchString(line) {
			suspected = append(suspected, SuspectedImport{Line: i, Content: line})
		}
	}
	return suspected
}
