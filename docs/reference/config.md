# Config file (`litprompt.yaml`)

Drop a `litprompt.yaml` (or `.yml`) in your repo and run `litprompt build` with no arguments to build every target at once. The same config drives `litprompt check`.

```yaml
builds:
  - source: agents-src/
    output: agents/
  - source: plugins/*/skills/*/SKILL.src.md
    output: SKILL.md
    header: full
```

## `builds`

A list of build entries. Each entry has a `source` and an `output`, plus optional per-build settings.

| Key | Required | Description |
|---|---|---|
| `source` | yes | A file, directory, or glob to build. |
| `output` | yes | Where the result goes. The allowed shape depends on the source shape (see below). |
| `header` | no | `short` or `full` — insert a generated-file comment after frontmatter. |
| `interlock` | no | `off`, `analytics`, or `enforce` — stamp an interlock line. |

Failed builds are reported but don't stop the others; `litprompt build` exits non-zero if any failed.

## Source/output shapes

Each source shape pairs with exactly one output shape:

| `source` shape | `output` shape | Behaviour |
|---|---|---|
| File (`prompt.md`) | Path (`out/prompt.md`) | Build to that path. |
| File (`prompt.md`) | Bare filename (`out.md`) | Write next to the source (sibling). |
| Directory (`src/`) | Directory (`out/`) | Mirror the tree of all `.md` files. |
| Glob (`a/*/b.md`) | Bare filename (`b.out.md`) | One sibling output per match. |

Globs use `**` for recursive matching. A glob source **must** pair with a bare-filename output — that keeps output paths predictable (one match → one sibling). Use directory mode if you want tree-mirroring.

## `interlock` block

An optional top-level block sets the shared interlock knobs for the whole run:

```yaml
interlock:
  param: interlock_tokens     # tool-parameter name in the line (default)
  manifest: interlocks.json   # one aggregate manifest for the run (default)
builds:
  - source: plugins/*/skills/*/SKILL.src.md
    output: SKILL.md
    interlock: enforce
```

| Key | Default | Description |
|---|---|---|
| `param` | `interlock_tokens` | The tool-parameter name referenced in the stamped line. |
| `manifest` | `interlocks.json` | Path to the single aggregate manifest, keyed by output path. |
| `message` | — | Override the stamped wording per mode, using `{token}` and `{param}` placeholders. |

See [Set up an interlock](../guides/interlocks.md) for the full workflow and [the interlock format](interlock-format.md) for the token/manifest schema.

## Selecting a config with `--config`

By default `litprompt build` / `litprompt check` discover `litprompt.yaml` in the current directory. Pass `--config <path>` to use a named file instead:

```sh
litprompt build --config litprompt.prod.yaml
litprompt check --config litprompt.staging.yaml
```

Sources, outputs, and the lockfile resolve **relative to the config file's directory**, so `--config envs/prod.yaml` behaves exactly like `cd envs && litprompt build` pointed at that file. `--config` cannot be combined with a source argument (that's single-file mode), and a missing named config is a hard error.

This is the basis for per-environment builds — see [Build for multiple environments](../guides/multiple-environments.md).

## Notes

- When building from a config, CLI flags `-o`, `--header`, and `--match` are ignored — per-build settings come from the file.
- `--vars` **is** honoured in config mode (variables are resolved at build time regardless of how the build was invoked).
