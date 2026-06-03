# Add litprompt to a repo with skills

This guide adds litprompt to an existing repository that already contains skills — for example a Claude Code plugin with several `SKILL.md` files — so you can author each skill with comments and shared imports, then build the clean `SKILL.md` the runtime loads.

The pattern is: keep an **authored source** next to each skill (`SKILL.src.md`), and build it into the `SKILL.md` that actually ships.

## 1. Rename your skills to source files

Say your repo looks like this:

```
plugins/
  data/
    skills/
      chart/SKILL.md
      query/SKILL.md
```

Rename each authored file to `SKILL.src.md`:

```
plugins/
  data/
    skills/
      chart/SKILL.src.md
      query/SKILL.src.md
```

`SKILL.src.md` is now the file you edit. `SKILL.md` will be generated — you'll never hand-edit it again.

## 2. Author with comments and imports

Now you can annotate freely and pull in shared fragments. For example, a shared safety block:

`shared/safety.md`:

```markdown
Never run destructive commands without explicit confirmation.
```

`plugins/data/skills/chart/SKILL.src.md`:

```markdown
---
name: Chart Builder
description: Generate charts from a spec.
---

<!-- @
Authoring note: keep this description under one line — the loader truncates.
-->

@[safety](../../../../shared/safety.md)

## How to build a chart
...
```

The comment is stripped at build time; the import is inlined.

## 3. Add a config file

Create `litprompt.yaml` at the repo root so every skill builds in one command:

```yaml
builds:
  - source: plugins/*/skills/*/SKILL.src.md
    output: SKILL.md
```

This is a **glob source paired with a bare-filename output** — each matched `SKILL.src.md` builds to a sibling `SKILL.md`. (Glob sources must use a bare filename output; see the [config reference](../reference/config.md) for the full shape matrix.)

Add `header: full` if you want each generated file to carry a "do not edit" banner:

```yaml
builds:
  - source: plugins/*/skills/*/SKILL.src.md
    output: SKILL.md
    header: full
```

## 4. Build

```sh
litprompt build
```

With no argument, litprompt discovers `litprompt.yaml` and builds every entry. Your `SKILL.md` files are regenerated next to their sources:

```
plugins/data/skills/chart/SKILL.src.md   # you edit this
plugins/data/skills/chart/SKILL.md        # generated — committed and shipped
```

## 5. Decide what to commit

Two common approaches:

- **Commit the built `SKILL.md`** (recommended for skills) so the runtime can load it directly without a build step, and so diffs of the shipped artifact are visible in review. Re-run `litprompt build` and commit both files when you change a source.
- **Gitignore the built files** and build them in CI/release if you'd rather treat them as pure artifacts.

If you commit the built files, add a CI check so they never drift from their sources:

```sh
litprompt build && git diff --exit-code
```

This rebuilds and fails if anything changed — i.e. someone edited a `SKILL.md` directly or forgot to rebuild.

## Next steps

- Validate everything resolves without writing output → `litprompt check` (see [CLI commands](../reference/cli.md)).
- Stamp interlock tokens so a tool can verify a skill was read → [Set up an interlock](interlocks.md).
- Share fragments across many skills → [Reuse fragments across prompts](reuse-fragments.md).
