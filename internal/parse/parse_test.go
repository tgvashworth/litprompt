package parse

import (
	"testing"
)

func TestStripComments_single(t *testing.T) {
	input := "# Hello\n\n<!-- @\nThis comment should be stripped.\n-->\n\nWorld.\n"
	want := "# Hello\n\nWorld.\n"
	got := StripComments(input)
	if got != want {
		t.Errorf("StripComments:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestStripComments_preserves_regular_html(t *testing.T) {
	input := "<!-- regular comment -->\n\n<!-- @\nstrip me\n-->\n\nContent.\n"
	want := "<!-- regular comment -->\n\nContent.\n"
	got := StripComments(input)
	if got != want {
		t.Errorf("StripComments:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestStripComments_collapses_blank_lines(t *testing.T) {
	input := "First.\n\n<!-- @\ncomment\n-->\n\nSecond.\n"
	want := "First.\n\nSecond.\n"
	got := StripComments(input)
	if got != want {
		t.Errorf("StripComments:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestStripComments_multi_blank_collapse(t *testing.T) {
	input := "First.\n\n\n<!-- @\ncomment\n-->\n\n\nSecond.\n"
	want := "First.\n\nSecond.\n"
	got := StripComments(input)
	if got != want {
		t.Errorf("StripComments:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestStripComments_empty_result(t *testing.T) {
	input := "<!-- @\nonly comments\n-->\n"
	want := ""
	got := StripComments(input)
	if got != want {
		t.Errorf("StripComments:\ngot:  %q\nwant: %q", got, want)
	}
}

func TestFindImports_basic(t *testing.T) {
	input := "# Main\n\n@[tone](./tone.md)\n\nDone.\n"
	imports := FindImports(input)
	if len(imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(imports))
	}
	if imports[0].Label != "tone" || imports[0].Target != "./tone.md" {
		t.Errorf("unexpected import: %+v", imports[0])
	}
	if imports[0].IsRemote() {
		t.Error("local import should not be remote")
	}
}

func TestFindImports_indented(t *testing.T) {
	input := "  @[tone](./tone.md)\n"
	imports := FindImports(input)
	if len(imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(imports))
	}
}

func TestFindImports_not_at_line_start(t *testing.T) {
	input := "text @[label](./file.md) more text\n"
	imports := FindImports(input)
	if len(imports) != 0 {
		t.Errorf("expected 0 imports for inline @[], got %d", len(imports))
	}
}

func TestFindImports_remote(t *testing.T) {
	input := "@[helper](https://github.com/org/repo/blob/abc/file.md)\n"
	imports := FindImports(input)
	if len(imports) != 1 {
		t.Fatalf("expected 1 import, got %d", len(imports))
	}
	if !imports[0].IsRemote() {
		t.Error("should be detected as remote")
	}
}

func TestFindSuspectedImports_trailing_content(t *testing.T) {
	input := "@[safety](./safety.md).\n"
	suspected := FindSuspectedImports(input)
	if len(suspected) != 1 {
		t.Fatalf("expected 1 suspected import, got %d", len(suspected))
	}
	if suspected[0].Line != 0 {
		t.Errorf("expected line 0, got %d", suspected[0].Line)
	}
}

func TestFindSuspectedImports_valid_import_not_suspected(t *testing.T) {
	input := "@[safety](./safety.md)\n"
	suspected := FindSuspectedImports(input)
	if len(suspected) != 0 {
		t.Errorf("valid import should not be suspected, got %d", len(suspected))
	}
}

func TestFindSuspectedImports_inline_not_suspected(t *testing.T) {
	input := "see @[this](./file.md) for more\n"
	suspected := FindSuspectedImports(input)
	if len(suspected) != 0 {
		t.Errorf("inline @[] should not be suspected, got %d", len(suspected))
	}
}
