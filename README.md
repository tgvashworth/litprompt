# litprompt

A markdown preprocessor for LLM prompts — strip comments, resolve imports, compose prompt systems from reusable parts.

## Example

**Source** (`prompt.md`):

```markdown
---
model: claude-4
---

# Coding assistant

<!-- @
Based on Q1 user research. The "rules" framing tested better than "guidelines".
See: https://internal.example.com/research/2026-q1
-->

@[tone](./shared/tone.md)

## Rules

1. Always explain your reasoning before writing code.
2. Prefer standard library solutions over third-party dependencies.

@[safety](https://github.com/acme/prompts/blob/v1.0/safety.md)
```

**Output** (`litprompt build prompt.md`):

```markdown
---
model: claude-4
---

# Coding assistant

Be direct and concise. Use a professional but approachable tone.

## Rules

1. Always explain your reasoning before writing code.
2. Prefer standard library solutions over third-party dependencies.

Do not execute arbitrary code or access external systems without explicit permission.
```

The comment is gone (saving tokens). The imports are inlined. The frontmatter is preserved.

## Why

- **Prompts are getting complex.** Agentic systems have dozens of prompts sharing common fragments. Copy-paste drift is a real source of bugs.
- **Comments waste tokens.** HTML comments stay in the markdown. `<!-- @ ... -->` comments are stripped at build time — annotate freely without cost.
- **Imports enable reuse.** Share tone, safety rules, or tool descriptions across prompts with `@[label](./path.md)`. The syntax degrades to a clickable link in any markdown renderer.
- **Remote imports are locked.** Import from other repos via URL. A `prompt.lock` with SHA-256 hashes ensures reproducibility and catches tampering.

## Install

```sh
go install github.com/tgvashworth/litprompt@latest
```

## Usage

```sh
litprompt build prompt.md                          # build one file, print to stdout
litprompt build prompt.md -o out.md                # write to a file
litprompt build prompts/ -o out/                   # build all .md files recursively
litprompt build prompts/ -o out/ --match '**/*.md' # filter which files to build
litprompt check prompt.md                          # validate: imports resolve, no cycles
litprompt lock                                     # fetch remote imports, write prompt.lock
cat prompt.md | litprompt build -                  # read from stdin
```

Flags: `-v` verbose, `-d` debug, `-q` quiet. The lockfile (`prompt.lock`) is discovered from the current working directory.

## Syntax

Two features, both using `@` applied to standard markdown constructs.

### Comments

```markdown
<!-- @
Author-only note. Stripped during build.
-->
```

- `<!-- @` opens a strippable comment. Regular `<!-- ... -->` comments pass through unchanged.
- Supports multi-line. Surrounding blank lines are collapsed.

### Imports

```markdown
@[tone](./shared/tone.md)
@[safety](https://github.com/acme/prompts/blob/v1.0/safety.md)
```

- Must be at the start of a line (leading whitespace is fine).
- Local paths are resolved relative to the importing file.
- Remote URLs are resolved from `prompt.lock` — run `litprompt lock` to fetch and hash them.
- YAML frontmatter is stripped from imported files. Only the root file's frontmatter is preserved.
- Imports are transitive (imported files can contain imports). Circular imports are detected.

## Lockfile

Remote imports require a `prompt.lock`:

```yaml
version: 1
imports:
  "https://github.com/acme/prompts/blob/v1.0/safety.md":
    hash: "sha256:e3b0c44..."
```

`litprompt lock` fetches every remote URL, computes its SHA-256 hash, and writes the lockfile. `litprompt build` verifies content against the lockfile — it never hits the network.

## Non-goals

- **No templating.** No variables, conditionals, or loops. Use a template engine upstream if you need these.
- **No partial imports.** You can't import a section of a file. Make your files granular instead.
- **No registry.** Git URLs are the distribution mechanism.
- **No rendering.** Output is markdown, not HTML.

## License

MIT
