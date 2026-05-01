package build

import (
	"regexp"
	"sort"
	"strings"
)

// varDirectivePattern matches @[placeholder](#NAME) where NAME is UPPER_SNAKE.
// Mirrors parse.varDirectivePattern but exported here as a private package-local
// to avoid an import cycle when the substitute pass is called from this package.
var varDirectivePattern = regexp.MustCompile(`@\[([^\]]+)\]\(#([A-Z_][A-Z0-9_]*)\)`)

// fenceLinePattern matches a line that opens or closes a fenced code block
// (``` or ~~~ with optional leading whitespace and an optional info string).
var fenceLinePattern = regexp.MustCompile("^\\s*(```|~~~)")

// SubstituteVars replaces variable directives in content with values from vars.
// Skips regions inside fenced code blocks (triple-backtick or triple-tilde) and
// inline single-backtick code spans.
//
// Returns the substituted content and a sorted, deduplicated list of directive
// names that had no value in vars. If vars is nil, every directive's name is
// reported as missing (this is the "no --vars supplied but directives present"
// case).
func SubstituteVars(content string, vars map[string]string) (string, []string) {
	missing := map[string]struct{}{}

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
		lines[i] = substituteOnLine(line, vars, missing)
	}

	if len(missing) == 0 {
		return strings.Join(lines, "\n"), nil
	}

	out := make([]string, 0, len(missing))
	for name := range missing {
		out = append(out, name)
	}
	sort.Strings(out)
	return strings.Join(lines, "\n"), out
}

// substituteOnLine walks a single line, replacing variable directives that
// fall outside inline code spans. Missing names are added to the missing set.
func substituteOnLine(line string, vars map[string]string, missing map[string]struct{}) string {
	if !strings.Contains(line, "@[") {
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
