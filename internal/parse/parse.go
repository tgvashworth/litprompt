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

// Var represents a variable directive @[placeholder](#NAME) found in the source.
type Var struct {
	Line        int
	Placeholder string
	Name        string
}

var (
	// commentPattern matches <!-- @ ... --> blocks (possibly multi-line),
	// including the trailing newline if present (the comment's own line ending).
	commentPattern = regexp.MustCompile(`(?s)<!-- @.*?-->\n?`)

	// importLinePattern matches @[label](target) at the start of a line,
	// with optional leading whitespace. A target matching the variable name
	// shape (#UPPER_SNAKE) is rejected by MatchImportLine.
	importLinePattern = regexp.MustCompile(`^\s*@\[([^\]]+)\]\(([^)]+)\)$`)

	// VarNameTargetPattern matches a variable-directive target: #UPPER_SNAKE.
	VarNameTargetPattern = regexp.MustCompile(`^#[A-Z_][A-Z0-9_]*$`)

	// varDirectivePattern matches @[placeholder](#NAME) anywhere on a line.
	varDirectivePattern = regexp.MustCompile(`@\[([^\]]+)\]\(#([A-Z_][A-Z0-9_]*)\)`)
)

// MatchImportLine returns the label and target if line is a valid import line.
// Returns ok=false if the line doesn't match the @[label](target) shape OR if
// the target matches the variable name shape (#UPPER_SNAKE) — variable
// directives are not imports.
func MatchImportLine(line string) (label, target string, ok bool) {
	m := importLinePattern.FindStringSubmatch(line)
	if m == nil {
		return "", "", false
	}
	if VarNameTargetPattern.MatchString(m[2]) {
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
// leading whitespace) and the target is not a variable name.
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

// FindVars returns all @[placeholder](#NAME) variable directives in content.
// Variable directives may appear anywhere on a line (unlike imports).
func FindVars(content string) []Var {
	var vars []Var
	lines := strings.Split(content, "\n")
	for i, line := range lines {
		for _, m := range varDirectivePattern.FindAllStringSubmatch(line, -1) {
			vars = append(vars, Var{Line: i, Placeholder: m[1], Name: m[2]})
		}
	}
	return vars
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
// Lines that contain a valid variable directive (e.g. `text @[U](#NAME) more`)
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
