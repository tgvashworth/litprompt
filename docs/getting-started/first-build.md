# Your first build

This walkthrough takes you from an empty folder to your first built prompt in a few minutes. You'll write a prompt that uses a comment and an import, then build it and see exactly what changed.

## Install

```sh
go install github.com/tgvashworth/litprompt@latest
```

This needs Go 1.24. Check it worked:

```sh
litprompt --version
```

## 1. Create a fragment to reuse

Make a folder and a shared fragment that more than one prompt might want:

```sh
mkdir prompts && cd prompts
```

`shared/tone.md`:

```markdown
Be direct and concise. Use a professional but approachable tone.
```

## 2. Write a prompt

`assistant.md`:

```markdown
---
model: claude-4
---

# Coding assistant

<!-- @
Author-only note: the "rules" framing tested better than "guidelines".
This comment will be stripped from the build output.
-->

@[tone](./shared/tone.md)

## Rules

1. Always explain your reasoning before writing code.
2. Prefer standard library solutions over third-party dependencies.
```

Three things are happening here:

- The `<!-- @ ... -->` block is an **author-only comment** — it stays in your source but is removed from the build.
- `@[tone](./shared/tone.md)` is an **import** — it will be replaced by the contents of `shared/tone.md`. In a plain markdown viewer it just looks like a link, so the source stays readable.
- The `---` frontmatter at the top is preserved in the output (imported files' frontmatter would be stripped — but `tone.md` has none).

## 3. Build it

```sh
litprompt build assistant.md
```

With no `-o`, the result prints to stdout:

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

The comment is gone, and the import line has become the fragment's content.

## 4. Write the output to a file

```sh
litprompt build assistant.md -o dist/assistant.md
```

`dist/assistant.md` now holds the built prompt — this is the file you'd ship to your model or agent runtime.

## 5. Validate without building

To check that everything resolves (imports exist, no circular references) without producing output:

```sh
litprompt check assistant.md
```

This is what you'd run in CI.

## Where to go next

- Reuse the same fragment across many prompts → [Reuse fragments across prompts](../guides/reuse-fragments.md).
- Build a whole tree of prompts at once with a config file → [Config file reference](../reference/config.md).
- Vary a prompt per environment → [Build for multiple environments](../guides/multiple-environments.md).
- The full directive rules → [Directive syntax](../reference/syntax.md).
