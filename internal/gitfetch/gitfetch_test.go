package gitfetch

import (
	"testing"
)

func TestParseGitURL_github(t *testing.T) {
	cloneURL, ref, path, err := ParseGitURL("https://github.com/org/repo/blob/main/path/file.md")
	if err != nil {
		t.Fatal(err)
	}
	if cloneURL != "https://github.com/org/repo.git" {
		t.Errorf("cloneURL = %q, want https://github.com/org/repo.git", cloneURL)
	}
	if ref != "main" {
		t.Errorf("ref = %q, want main", ref)
	}
	if path != "path/file.md" {
		t.Errorf("path = %q, want path/file.md", path)
	}
}

func TestParseGitURL_github_commit(t *testing.T) {
	cloneURL, ref, path, err := ParseGitURL("https://github.com/incident-io/internal-ai/blob/1ba804d/agents/delegator/prompt.md")
	if err != nil {
		t.Fatal(err)
	}
	if cloneURL != "https://github.com/incident-io/internal-ai.git" {
		t.Errorf("cloneURL = %q", cloneURL)
	}
	if ref != "1ba804d" {
		t.Errorf("ref = %q, want 1ba804d", ref)
	}
	if path != "agents/delegator/prompt.md" {
		t.Errorf("path = %q", path)
	}
}

func TestParseGitURL_github_deep_path(t *testing.T) {
	cloneURL, ref, path, err := ParseGitURL("https://github.com/org/repo/blob/v1.2.0/a/b/c/d.md")
	if err != nil {
		t.Fatal(err)
	}
	if cloneURL != "https://github.com/org/repo.git" {
		t.Errorf("cloneURL = %q", cloneURL)
	}
	if ref != "v1.2.0" {
		t.Errorf("ref = %q, want v1.2.0", ref)
	}
	if path != "a/b/c/d.md" {
		t.Errorf("path = %q, want a/b/c/d.md", path)
	}
}

func TestParseGitURL_gitlab(t *testing.T) {
	cloneURL, ref, path, err := ParseGitURL("https://gitlab.com/org/repo/-/blob/main/prompts/tone.md")
	if err != nil {
		t.Fatal(err)
	}
	if cloneURL != "https://gitlab.com/org/repo.git" {
		t.Errorf("cloneURL = %q", cloneURL)
	}
	if ref != "main" {
		t.Errorf("ref = %q", ref)
	}
	if path != "prompts/tone.md" {
		t.Errorf("path = %q", path)
	}
}

func TestParseGitURL_bitbucket(t *testing.T) {
	cloneURL, ref, path, err := ParseGitURL("https://bitbucket.org/org/repo/src/main/prompts/tone.md")
	if err != nil {
		t.Fatal(err)
	}
	if cloneURL != "https://bitbucket.org/org/repo.git" {
		t.Errorf("cloneURL = %q", cloneURL)
	}
	if ref != "main" {
		t.Errorf("ref = %q", ref)
	}
	if path != "prompts/tone.md" {
		t.Errorf("path = %q", path)
	}
}

func TestParseGitURL_invalid(t *testing.T) {
	_, _, _, err := ParseGitURL("https://example.com/not/a/git/url")
	if err == nil {
		t.Error("expected error for unrecognized URL")
	}
}

func TestParseGitURL_not_https(t *testing.T) {
	_, _, _, err := ParseGitURL("http://github.com/org/repo/blob/main/file.md")
	if err == nil {
		t.Error("expected error for non-https URL")
	}
}
