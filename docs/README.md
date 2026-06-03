# litprompt

A build system for prompts and skills: strip comments, resolve imports, substitute variables, and compose prompt systems from reusable parts.

![intro gif](https://raw.githubusercontent.com/tgvashworth/litprompt/main/assets/litprompt-3.gif)

litprompt takes markdown you write for humans and produces the flattened markdown an LLM should actually receive. It does three things and nothing more: it **strips author-only comments**, **inlines imports** (local files and pinned remote URLs), and **substitutes variables** from `.env`-style files at build time.

## Example

**Source** (`prompt.md`):

```markdown
---
model: claude-4
---

# Coding assistant

<!-- @
Based on Q1 user research. The "rules" framing tested better than "guidelines".
-->

@[tone](./shared/tone.md)

## Rules

1. Always explain your reasoning before writing code.
2. Prefer standard library solutions over third-party dependencies.
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
```

The comment is gone (saving tokens). The import is inlined. The frontmatter is preserved.

## Why

- **Prompts are getting complex.** Agentic systems have dozens of prompts sharing common fragments. Copy-paste drift is a real source of bugs.
- **Comments waste tokens.** `<!-- @ ... -->` comments are stripped at build time — annotate freely without cost.
- **Imports enable reuse.** Share tone, safety rules, or tool descriptions across prompts with `@[label](./path.md)`. The syntax degrades to a clickable link in any markdown renderer.
- **Variables handle environments.** Swap values per environment with `--vars`, while the raw file still shows a sensible default.
- **Remote imports are locked.** Import from other repos by URL; a `litprompt.lock` with SHA-256 hashes ensures reproducibility and catches tampering.

For the design philosophy and what litprompt deliberately leaves out, see [Why litprompt](concepts/why.md).

## Install

```sh
go install github.com/tgvashworth/litprompt@latest
```

Requires Go 1.24.

## Where to go next

- **New here?** Start with [Your first build](getting-started/first-build.md).
- **Adding it to a project?** [Add litprompt to a repo with skills](guides/add-to-skills-repo.md).
- **Need prod/staging variants?** [Build for multiple environments](guides/multiple-environments.md).
- **Looking something up?** [CLI commands](reference/cli.md), [the config file](reference/config.md), and [directive syntax](reference/syntax.md).

## Interlocks

An [interlock](https://tgvashworth.com/2026/05/25/skill-tool-interlock.html) stamps a token into a built skill so a paired tool can tell whether the skill was actually read. litprompt only stamps the token and emits a manifest — the tool that logs or enforces it lives elsewhere.

Turn it on per build with `interlock: analytics` or `interlock: enforce` in your config. Each built file gets a line after its frontmatter:

```
Interlock: `chart-builder:db13d3df:fed100da` — you MUST pass this as `interlock_tokens` when you call the tool, or the call will be rejected.
```

See [Set up an interlock](guides/interlocks.md) for the build side and [Consume interlocks in a tool](guides/interlock-consumer.md) for the tool side.

## License

MIT
