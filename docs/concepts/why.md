# Why litprompt

litprompt exists because prompts have outgrown the single-file, copy-paste era. An agentic system today is dozens of prompts and skills that share tone, safety rules, tool descriptions, and boilerplate. Keeping those fragments in sync by hand is the same problem build tools solved for source code — so litprompt treats prompts like source: you author them for humans, and *build* the artifact the model receives.

## The shape of the tool

litprompt is a **preprocessor**, not a template engine. It does exactly three transformations:

1. **Strip comments** — `<!-- @ ... -->` blocks are author-only and removed from the output, so you can annotate freely without spending tokens or leaking notes to the model.
2. **Resolve imports** — `@[label](path-or-url)` is replaced with the content of another file, local or remote. Reuse without duplication.
3. **Substitute variables** — `[{{default}}](#NAME)` is replaced with a value from a `--vars` file at build time, so one source tree can target many environments.

Everything else is out of scope on purpose. The value is in the *constraints*: the source is always valid, renderable markdown, and the output is always plain markdown.

## Variables are substitution, not templating

litprompt has variables, but it does **not** have a template language. The distinction is deliberate:

- A variable is written `[{{default}}](#NAME)` — an ordinary markdown link. The `{{default}}` text renders as a sensible value when the raw file is viewed in any markdown viewer or read by a local agent; the `#NAME` target says which variable supplies the real value at build time.
- There are **no conditionals, no loops, no interpolation, no expressions**. A variable is a named hole that gets one string substituted into it.
- **Missing variables are a hard error**, never a silent empty string — a typo fails the build instead of shipping the wrong prompt to production.

This buys per-environment values (a Slack bot ID, a channel name, a model) without the failure modes of a full template engine, and without making the source unreadable.

## Non-goals

litprompt intentionally excludes:

- **Conditionals, loops, and expression templating.** If you need generated structure, run a template engine *upstream* and feed its output to litprompt.
- **Partial imports.** You can't import a section of a file. Make your files granular instead — one fragment per file.
- **A package registry.** Git URLs are the distribution mechanism. Remote imports are pinned by content hash in `litprompt.lock`.
- **Rendering.** Output is markdown, not HTML. Rendering is someone else's job.

## How it degrades

A litprompt source file is still a valid markdown document *before* it is built:

- Imports look like ordinary links — clickable in any renderer, and followable by a human or agent reading the repo.
- Variables look like a linked default value — you see `U1234` linked to `#BOT_ID`, not an opaque token.
- Comments are standard HTML comments.

That graceful degradation is the point: the thing you commit is readable on its own, and the build only sharpens it for the model.

## See also

- [How imports resolve](imports.md) — transitivity, frontmatter handling, and circular-import detection.
- [Directive syntax](../reference/syntax.md) — the precise rules for all three directives.
