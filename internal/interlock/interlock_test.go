package interlock

import (
	"strings"
	"testing"
)

func TestSlugify(t *testing.T) {
	cases := []struct{ in, want string }{
		{"Coding Assistant", "coding-assistant"},
		{"  Trimmed  ", "trimmed"},
		{"UPPER_case Mix", "upper-case-mix"},
		{"weird---chars!!!here", "weird-chars-here"},
		{"--leading-trailing--", "leading-trailing"},
		{"already-slug", "already-slug"},
		{"123 numbers 456", "123-numbers-456"},
	}
	for _, c := range cases {
		if got := Slugify(c.in); got != c.want {
			t.Errorf("Slugify(%q) = %q, want %q", c.in, got, c.want)
		}
	}
}

func TestDeriveIdentity_interlockWinsOverName(t *testing.T) {
	src := "---\nname: Coding Assistant\ninterlock: ca-prod-1\n---\nbody\n"
	id, err := DeriveIdentity(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.IdentitySource != "interlock" {
		t.Errorf("identitySource = %q, want interlock", id.IdentitySource)
	}
	if id.ID != shortHash("ca-prod-1") {
		t.Errorf("id = %q, want hash of pinned value", id.ID)
	}
	// task still derives from the display name.
	if id.Task != "coding-assistant" {
		t.Errorf("task = %q, want coding-assistant", id.Task)
	}
	if id.Name != "Coding Assistant" {
		t.Errorf("name = %q, want raw name preserved", id.Name)
	}
}

func TestDeriveIdentity_nameOnly(t *testing.T) {
	src := "---\nname: Chart Builder\n---\nbody\n"
	id, err := DeriveIdentity(src)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if id.IdentitySource != "name" {
		t.Errorf("identitySource = %q, want name", id.IdentitySource)
	}
	if id.ID != shortHash("chart-builder") {
		t.Errorf("id = %q, want hash of slug", id.ID)
	}
	if id.Task != "chart-builder" {
		t.Errorf("task = %q, want chart-builder", id.Task)
	}
}

func TestDeriveIdentity_neither_errors(t *testing.T) {
	for _, src := range []string{
		"no frontmatter here\n",
		"---\ndescription: just a description\n---\nbody\n",
		"",
	} {
		if _, err := DeriveIdentity(src); err == nil {
			t.Errorf("expected error for source %q, got nil", src)
		}
	}
}

func TestDeriveIdentity_distinctNamesDistinctIDs(t *testing.T) {
	// The SKILL.md collision fix: identical bodies, different names → different ids.
	a, _ := DeriveIdentity("---\nname: Chart Builder\n---\nbody\n")
	b, _ := DeriveIdentity("---\nname: Query Runner\n---\nbody\n")
	if a.ID == b.ID {
		t.Errorf("expected distinct ids, both = %q", a.ID)
	}
}

func TestVersion_stableAndContentSensitive(t *testing.T) {
	if Version("hello") != Version("hello") {
		t.Error("Version not stable for identical body")
	}
	if Version("hello") == Version("hello!") {
		t.Error("Version did not change when body changed")
	}
	if len(Version("hello")) != 8 {
		t.Errorf("Version length = %d, want 8", len(Version("hello")))
	}
}

func TestLine_defaults(t *testing.T) {
	analytics := Line("t:i:v", "interlock_tokens", ModeAnalytics, nil)
	if !strings.Contains(analytics, "`t:i:v`") || !strings.Contains(analytics, "`interlock_tokens`") {
		t.Errorf("analytics line missing token/param: %q", analytics)
	}
	if strings.Contains(analytics, "MUST") {
		t.Errorf("analytics line should not be mandatory: %q", analytics)
	}

	enforce := Line("t:i:v", "interlock_tokens", ModeEnforce, nil)
	if !strings.Contains(enforce, "MUST") || !strings.Contains(enforce, "rejected") {
		t.Errorf("enforce line should be mandatory: %q", enforce)
	}
}

func TestLine_customMessage(t *testing.T) {
	msg := map[string]string{ModeEnforce: "pass {token} via {param}"}
	got := Line("a:b:c", "myparam", ModeEnforce, msg)
	if got != "pass a:b:c via myparam" {
		t.Errorf("custom message not substituted: %q", got)
	}
}

func TestMarshal_versionedAndSorted(t *testing.T) {
	entries := map[string]ManifestEntry{
		"z.md": {Task: "z", ID: "2", Version: "v", Name: "Z", IdentitySource: "name"},
		"a.md": {Task: "a", ID: "1", Version: "v", Name: "A", IdentitySource: "name"},
	}
	data, err := Marshal(entries)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	out := string(data)
	if !strings.Contains(out, `"version": 1`) {
		t.Errorf("manifest missing schema version: %s", out)
	}
	if !strings.HasSuffix(out, "\n") {
		t.Error("manifest should end with a newline")
	}
	// encoding/json sorts map keys, so a.md precedes z.md.
	if strings.Index(out, "a.md") > strings.Index(out, "z.md") {
		t.Errorf("manifest keys not sorted: %s", out)
	}
}
