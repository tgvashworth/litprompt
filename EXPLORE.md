# Literate prompting spec

## Syntax

Two features, unified by `@` as the directive character, applied to existing markdown constructs.

### Comments

```markdown
<!-- @
Author-only commentary. Stripped during build.
Multiple lines are fine.
-->
```

- Must be a standard HTML comment with `@` immediately after `<!--`
- Stripped entirely from build output (including any surrounding blank lines left behind)
- Regular HTML comments (`<!-- ... -->` without `@`) are passed through unchanged

### Imports

```markdown
@[label](./relative/path.md)
```

- Must appear at the start of a line (no leading content, leading whitespace is fine)
- The link text is a human-readable label — not semantically meaningful to the build
- The path is resolved relative to the importing file
- Imported content replaces the `@[...]()` line in the output
- YAML frontmatter in imported files is stripped (only the root file's frontmatter is preserved)
- Transitive imports are resolved (imported files can themselves contain `@[...]()` imports)
- Circular imports are detected and rejected with an error

### Remote imports

```markdown
@[delegator](https://github.com/incident-io/internal-ai/blob/1ba804d/agents/delegator/prompt.md)
```

- Same syntax as local imports, but with an HTTPS URL
- The build tool parses the URL to extract repo, ref, and path
- Supported hosts (initially): GitHub, GitLab, Bitbucket — normalised internally to `(host, owner, repo, ref, path)`
- Fetched using ambient git credentials (local git config, credential helpers)
- Requires the URL to be present in `prompt.lock` with a matching content hash — if missing, the build fails with a message to run `litprompt lock`
- Remote imports without a lockfile entry are never fetched implicitly

## Lockfile

`prompt.lock` — maps import URLs to content hashes.

```yaml
version: 1
imports:
  "https://github.com/incident-io/internal-ai/blob/1ba804d/agents/delegator/prompt.md":
    hash: "sha256:e3b0c44298fc1c149afbf4c8996fb92427ae41e4649b934ca495991b7852b855"
  "https://github.com/incident-io/prompts/blob/v1.2.0/shared/tone.md":
    hash: "sha256:abc123..."
```

- URL is the key (exactly as written in the source file)
- Value is a SHA-256 hash of the file content at that ref
- `litprompt lock` fetches all remote imports, computes hashes, writes/updates `prompt.lock`
- `litprompt build` verifies fetched content against the lockfile hash — fails on mismatch
- Local imports are not lockfile-tracked (they're already version-controlled in the same repo)

## CLI

```
litprompt build <file.md>          # build one file, output to stdout
litprompt build <file.md> -o out/  # build to output directory
litprompt build <dir/>             # build all .md files in directory
litprompt lock                     # fetch remote imports, update prompt.lock
litprompt check                    # lint: validate imports resolve, lockfile is current, no cycles
```

## Caching

- Remote content cached in `~/.cache/litprompt/` by content hash
- After first fetch, builds work offline from cache
- `litprompt lock` is the only command that hits the network

## Non-goals (intentionally excluded)

- Variables, conditionals, loops, templating — use a template engine upstream if you need these
- Partial file imports (import a section) — make files granular instead
- A package registry — git URLs are the distribution mechanism
- Markdown rendering/conversion — output is markdown, not HTML
