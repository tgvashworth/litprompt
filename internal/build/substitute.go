package build

import (
	"regexp"
	"sort"
	"strings"
)

// varDirectivePattern matches [{{placeholder}}](#NAME). Unlike
// parse.varDirectivePattern (which is strict UPPER_SNAKE), the name group here
// is intentionally loose (any case) so a misspelled lowercase name can be
// detected and reported rather than silently passing through unsubstituted;
// substituteOnLine then classifies each match via upperSnakePattern. Kept
// package-local to avoid an import cycle on the parse package.
var varDirectivePattern = regexp.MustCompile(`\[\{\{([^}]*)\}\}\]\(#([A-Za-z_][A-Za-z0-9_]*)\)`)

// upperSnakePattern reports whether a variable name has the required
// UPPER_SNAKE_CASE shape.
var upperSnakePattern = regexp.MustCompile(`^[A-Z_][A-Z0-9_]*$`)

// fenceLinePattern matches a line that opens or closes a fenced code block
// (``` or ~~~ with optional leading whitespace and an optional info string).
var fenceLinePattern = regexp.MustCompile("^\\s*(```|~~~)")

// SubstituteVars replaces variable directives [{{placeholder}}](#NAME) in
// content with values from vars. Skips regions inside fenced code blocks
// (triple-backtick or triple-tilde) and inline single-backtick code spans.
//
// Returns the substituted content, a sorted/deduplicated list of UPPER_SNAKE
// directive names that had no value in vars (missing), and a sorted/deduplicated
// list of directive names that were not UPPER_SNAKE (miscased — a likely typo).
// If vars is nil, every UPPER_SNAKE directive's name is reported as missing
// (this is the "no --vars supplied but directives present" case).
func SubstituteVars(content string, vars map[string]string) (out string, missing, miscased []string) {
	missingSet := map[string]struct{}{}
	miscasedSet := map[string]struct{}{}

	lines := strings.Split(content, "\n")
	inFence := false

	for i, line := range lines {
		if fenceLinePattern.MatchString(line) {
			inFence = !inFence
			continue
		}
		if inFence {
			continue
		}
		lines[i] = substituteOnLine(line, vars, missingSet, miscasedSet)
	}

	return strings.Join(lines, "\n"), sortedKeys(missingSet), sortedKeys(miscasedSet)
}

// sortedKeys returns the keys of set sorted, or nil if set is empty.
func sortedKeys(set map[string]struct{}) []string {
	if len(set) == 0 {
		return nil
	}
	out := make([]string, 0, len(set))
	for k := range set {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// substituteOnLine walks a single line, replacing variable directives that
// fall outside inline code spans. Missing UPPER_SNAKE names are added to the
// missing set; directives whose name is not UPPER_SNAKE are added to miscased.
func substituteOnLine(line string, vars map[string]string, missing, miscased map[string]struct{}) string {
	if !strings.Contains(line, "[{{") {
		return line
	}

	inSpan := buildInSpanMask(line)

	matches := varDirectivePattern.FindAllStringSubmatchIndex(line, -1)
	if len(matches) == 0 {
		return line
	}

	// Replace right-to-left so earlier-index matches keep their positions.
	out := line
	for i := len(matches) - 1; i >= 0; i-- {
		m := matches[i]
		start, end := m[0], m[1]
		nameStart, nameEnd := m[4], m[5]

		if start < len(inSpan) && inSpan[start] {
			continue
		}

		name := line[nameStart:nameEnd]
		if !upperSnakePattern.MatchString(name) {
			miscased[name] = struct{}{}
			continue
		}
		value, ok := lookup(vars, name)
		if !ok {
			missing[name] = struct{}{}
			continue
		}
		out = out[:start] + value + out[end:]
	}
	return out
}

// lookup returns the value for name in vars, or false if absent.
// A nil vars map always reports missing — used to enforce the
// "directives present but --vars not supplied" error path.
func lookup(vars map[string]string, name string) (string, bool) {
	if vars == nil {
		return "", false
	}
	v, ok := vars[name]
	return v, ok
}

// buildInSpanMask returns a per-byte slice where mask[i] is true iff position i
// falls inside an inline single-backtick code span. Backticks themselves are
// considered outside the span (they're the delimiters).
func buildInSpanMask(line string) []bool {
	mask := make([]bool, len(line))
	inside := false
	for i := 0; i < len(line); i++ {
		if line[i] == '`' {
			// Backtick toggles the state; the backtick itself is a boundary.
			mask[i] = false
			inside = !inside
			continue
		}
		mask[i] = inside
	}
	return mask
}
