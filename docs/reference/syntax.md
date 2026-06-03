# Directive syntax

litprompt has three directives, each layered onto a standard markdown construct. Comments and imports use `@`; variables are an ordinary link whose text is wrapped in `{{ }}`. All three leave the source as valid, readable markdown.

## Comments

```markdown
<!-- @
Author-only note. Stripped during build.
Multiple lines are fine.
-->
```

- A strippable comment is a standard HTML comment with `@` immediately after `<!--`.
- It is removed entirely from the build output, including surrounding blank lines left behind.
- Regular HTML comments (`<!-- ... -->` without the `@`) pass through unchanged.

## Imports

```markdown
@[tone](./shared/tone.md)
@[safety](https://github.com/acme/prompts/blob/v1.0/safety.md)
```

- Must appear at the **start of a line** (leading whitespace is fine).
- The link text (`tone`, `safety`) is a human-readable label — not semantically meaningful to the build.
- The `@[...]()` line is replaced by the imported file's content.
- **Local paths** resolve relative to the importing file.
- **Remote URLs** resolve from `litprompt.lock` — run `litprompt lock` to fetch and hash them first. Remote imports are never fetched implicitly during a build. See the [lockfile reference](lockfile.md).
- **Frontmatter is stripped** from imported files (including the blank line after the closing `---`). Only the root file's frontmatter is preserved, so the output has at most one frontmatter block.
- Imports are **transitive** — imported files may themselves contain imports. **Circular imports are detected** and rejected with an error showing the cycle.

See [How imports resolve](../concepts/imports.md) for the resolution model.

## Variables

```markdown
The bot is [{{`U1234`}}](#BOT_ID) in channel [{{`#general`}}](#CHANNEL).
```

Supplied at build time from one or more `--vars` files:

```sh
litprompt build prompt.md --vars prod.env
litprompt build prompt.md --vars base.env --vars prod.env  # later overrides earlier
```

```sh
# prod.env
BOT_ID=U99FOO
CHANNEL=#alerts
```

Rules:

- A variable is written `[{{placeholder}}](#NAME)` — an ordinary markdown link whose text is wrapped in `{{ }}`. The `{{placeholder}}` text is the default shown in the raw source, so a markdown viewer or a local agent reading the file sees a sensible value and a link to `#NAME`.
- The target must match `#NAME` where `NAME` is `UPPER_SNAKE_CASE` (`[A-Z_][A-Z0-9_]*`). A `{{ }}`-wrapped directive whose name isn't `UPPER_SNAKE_CASE` is reported as a likely typo.
- Variables can appear anywhere on a line, multiple times per line.
- Directives inside fenced code blocks (` ``` `, `~~~`) and inline code spans (`` `…` ``) are preserved verbatim — safe to document the syntax in your own prompts.
- Variables compose with imports: an imported fragment sees the parent build's variables without redeclaring them.
- **Missing variables are a hard build error.** A typo or unset value fails loudly rather than silently rendering an empty string into the wrong environment.

### `.env` file format

`KEY=value`, `KEY="value with spaces"`, `KEY='also fine'`, `# comments`, blank lines, and CRLF are all supported. A trailing `# comment` is stripped from unquoted values. There is **no interpolation, no `export`, and no multi-line values**.

See [Parameterise prompts with variables](../guides/variables.md) for a task-oriented walkthrough.
