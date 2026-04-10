package gitfetch

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Supported URL patterns:
//   GitHub:    https://github.com/{owner}/{repo}/blob/{ref}/{path...}
//   GitLab:    https://gitlab.com/{owner}/{repo}/-/blob/{ref}/{path...}
//   Bitbucket: https://bitbucket.org/{owner}/{repo}/src/{ref}/{path...}

var (
	githubRe    = regexp.MustCompile(`^https://github\.com/([^/]+/[^/]+)/blob/([^/]+)/(.+)$`)
	gitlabRe    = regexp.MustCompile(`^https://gitlab\.com/([^/]+/[^/]+)/-/blob/([^/]+)/(.+)$`)
	bitbucketRe = regexp.MustCompile(`^https://bitbucket\.org/([^/]+/[^/]+)/src/([^/]+)/(.+)$`)
)

// ParseGitURL extracts a clone URL, ref, and file path from a GitHub/GitLab/Bitbucket blob URL.
func ParseGitURL(rawURL string) (cloneURL, ref, path string, err error) {
	if !strings.HasPrefix(rawURL, "https://") {
		return "", "", "", fmt.Errorf("unsupported URL scheme (must be https): %s", rawURL)
	}

	for _, pattern := range []struct {
		re   *regexp.Regexp
		host string
	}{
		{githubRe, "github.com"},
		{gitlabRe, "gitlab.com"},
		{bitbucketRe, "bitbucket.org"},
	} {
		matches := pattern.re.FindStringSubmatch(rawURL)
		if matches != nil {
			repo := matches[1]
			return fmt.Sprintf("https://%s/%s.git", pattern.host, repo), matches[2], matches[3], nil
		}
	}

	return "", "", "", fmt.Errorf("unsupported git URL format: %s", rawURL)
}

// FetchFile fetches a single file from a git repo at a specific ref using the git CLI.
// Tries the given clone URL first (HTTPS), then falls back to SSH if HTTPS auth fails.
func FetchFile(cloneURL, ref, filePath string) (string, error) {
	// Try HTTPS first, then SSH fallback.
	urls := []string{cloneURL}
	if sshURL := toSSHURL(cloneURL); sshURL != "" {
		urls = append(urls, sshURL)
	}

	var lastErr error
	for _, url := range urls {
		content, err := fetchFileFromRemote(url, ref, filePath)
		if err == nil {
			return content, nil
		}
		lastErr = err
	}
	return "", lastErr
}

func fetchFileFromRemote(cloneURL, ref, filePath string) (string, error) {
	tmpDir, err := os.MkdirTemp("", "litprompt-fetch-*")
	if err != nil {
		return "", fmt.Errorf("creating temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	cmds := []struct {
		args []string
		desc string
	}{
		{[]string{"init"}, "git init"},
		{[]string{"remote", "add", "origin", cloneURL}, "git remote add"},
		{[]string{"fetch", "--depth=1", "origin", ref}, "git fetch"},
	}

	for _, c := range cmds {
		cmd := exec.Command("git", c.args...)
		cmd.Dir = tmpDir
		cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
		if out, err := cmd.CombinedOutput(); err != nil {
			return "", fmt.Errorf("%s failed for %s: %w\n%s", c.desc, cloneURL, err, string(out))
		}
	}

	showCmd := exec.Command("git", "show", fmt.Sprintf("FETCH_HEAD:%s", filePath))
	showCmd.Dir = tmpDir
	out, err := showCmd.Output()
	if err != nil {
		return "", fmt.Errorf("git show %s failed: %w", filePath, err)
	}

	content := string(out)
	if len(content) > 0 && !strings.HasSuffix(content, "\n") {
		content += "\n"
	}

	return content, nil
}

// toSSHURL converts an HTTPS clone URL to its SSH equivalent.
// e.g. "https://github.com/org/repo.git" → "git@github.com:org/repo.git"
// Returns "" if the URL can't be converted.
func toSSHURL(httpsURL string) string {
	// Match https://{host}/{path}.git
	prefix := "https://"
	if !strings.HasPrefix(httpsURL, prefix) {
		return ""
	}
	rest := strings.TrimPrefix(httpsURL, prefix)
	slash := strings.Index(rest, "/")
	if slash < 0 {
		return ""
	}
	host := rest[:slash]
	path := rest[slash+1:]
	return fmt.Sprintf("git@%s:%s", host, path)
}

// CacheDir returns the default cache directory for litprompt.
func CacheDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return filepath.Join(os.TempDir(), "litprompt-cache")
	}
	return filepath.Join(home, ".cache", "litprompt")
}
