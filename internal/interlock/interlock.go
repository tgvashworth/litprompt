// Package interlock derives skill-tool interlock tokens and the aggregate
// manifest. A token is the triple slug:id:version, stamped into a built file so
// a consuming tool can observe (analytics) or require (enforce) that the file
// was read. litprompt only stamps tokens and emits the manifest; the tool side
// that logs or enforces them lives elsewhere.
//
// The package is pure: it takes source/body content as strings and does no I/O.
package interlock

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"go.yaml.in/yaml/v3"
)

// Consumption modes. analytics logs tokens; enforce hard-rejects unknown ids.
const (
	ModeOff       = "off"
	ModeAnalytics = "analytics"
	ModeEnforce   = "enforce"
)

var (
	// frontmatterRe captures the inner YAML of a leading frontmatter block.
	frontmatterRe = regexp.MustCompile(`(?s)\A---\n(.*?)\n---\n?`)
	nonSlugRe     = regexp.MustCompile(`[^a-z0-9]+`)
)

// Slugify lowercases s, collapses each run of non-[a-z0-9] into a single dash,
// and strips leading/trailing dashes. Deterministic and locale-independent.
func Slugify(s string) string {
	s = strings.ToLower(strings.TrimSpace(s))
	s = nonSlugRe.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}

// shortHash returns the first 8 hex chars of sha256(s). This is the single home
// for the 8-hex truncation shared by id and version.
func shortHash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)[:8]
}

// Identity is the interlock identity derived from a source file's frontmatter.
type Identity struct {
	Slug           string // display slug — advisory, never verified
	ID             string // 8-hex stable id — the verification key
	Name           string // raw name: frontmatter value, if present
	IdentitySource string // "name" | "interlock"
}

// Token returns the slug:id:version triple for this identity.
func (id Identity) Token(version string) string {
	return fmt.Sprintf("%s:%s:%s", id.Slug, id.ID, version)
}

type frontmatterFields struct {
	Name      string `yaml:"name"`
	Interlock string `yaml:"interlock"`
}

// DeriveIdentity reads a source file's frontmatter and derives its identity. An
// explicit `interlock:` value wins over `name:` and lets the display name be
// renamed without rotating the id. With neither key present it is a hard error
// (the caller adds the source path for context).
func DeriveIdentity(source string) (Identity, error) {
	var fm frontmatterFields
	if inner := frontmatterRe.FindStringSubmatch(source); inner != nil {
		if err := yaml.Unmarshal([]byte(inner[1]), &fm); err != nil {
			return Identity{}, fmt.Errorf("parsing frontmatter: %w", err)
		}
	}

	name := strings.TrimSpace(fm.Name)
	pinned := strings.TrimSpace(fm.Interlock)

	switch {
	case pinned != "":
		// The id derives from the raw pinned value, so it stays unique even
		// when the value has no slug-safe characters.
		return Identity{Slug: displaySlug(name, pinned), ID: shortHash(pinned), Name: name, IdentitySource: "interlock"}, nil
	case name != "":
		slug := Slugify(name)
		if slug == "" {
			// The id would derive from an empty slug, colliding with every
			// other such name. Refuse it and point at the explicit escape hatch.
			return Identity{}, fmt.Errorf("name %q has no slug-safe characters; add an 'interlock:' frontmatter field to set an explicit id", name)
		}
		return Identity{Slug: slug, ID: shortHash(slug), Name: name, IdentitySource: "name"}, nil
	default:
		return Identity{}, fmt.Errorf("add a 'name:' or 'interlock:' frontmatter field")
	}
}

// displaySlug is slugify(name) when a name is present, else slugify(identity).
func displaySlug(name, identity string) string {
	if name != "" {
		return Slugify(name)
	}
	return Slugify(identity)
}

// Version returns the 8-hex content hash of a built body (frontmatter already
// stripped). It is a staleness signal only and is never enforced.
func Version(body string) string {
	return shortHash(body)
}

var defaultMessages = map[string]string{
	ModeAnalytics: "Interlock: `{token}` — include this in `{param}` when you call the tool.",
	ModeEnforce:   "Interlock: `{token}` — you MUST pass this as `{param}` when you call the tool, or the call will be rejected.",
}

// Line builds the emitted interlock sentence for mode. A non-empty
// message[mode] overrides the default wording; {token} and {param} are
// substituted in either case.
func Line(token, param, mode string, message map[string]string) string {
	tmpl := message[mode]
	if tmpl == "" {
		tmpl = defaultMessages[mode]
	}
	return strings.NewReplacer("{token}", token, "{param}", param).Replace(tmpl)
}

// ManifestEntry is one skill's interlock record, keyed by output path.
type ManifestEntry struct {
	Slug           string `json:"slug"`
	ID             string `json:"id"`
	Version        string `json:"version"`
	Name           string `json:"name"`
	IdentitySource string `json:"identitySource"`
}

// Manifest is the aggregate interlock manifest for a build run.
type Manifest struct {
	Version    int                      `json:"version"`
	Interlocks map[string]ManifestEntry `json:"interlocks"`
}

// Marshal renders the entries as a versioned, indented JSON manifest with a
// trailing newline. Map keys are emitted in sorted order by encoding/json.
func Marshal(entries map[string]ManifestEntry) ([]byte, error) {
	data, err := json.MarshalIndent(Manifest{Version: 1, Interlocks: entries}, "", "  ")
	if err != nil {
		return nil, err
	}
	return append(data, '\n'), nil
}
