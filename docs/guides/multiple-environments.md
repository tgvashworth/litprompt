# Build for multiple environments

You often need the same prompts in more than one flavour: a staging build that points at a test bot and a prod build that points at the real one, or per-customer variants that differ only in a few values. litprompt handles this with two mechanisms that compose:

- **`--config <path>`** — pick a named config file, so each environment has its own list of build targets and output paths.
- **`--vars <file>`** — substitute environment-specific values into otherwise-identical sources.

Keep **one source tree**. The environments differ only in their config and their vars file — never in the prompt content itself.

## Approach 1: vary values with `--vars`

If your environments differ only in a handful of values (IDs, channels, model names), put those values behind variables in your source and supply them per environment.

In your prompt, write each value as a variable — an ordinary markdown link whose text is the default:

```markdown
Post incidents to [{{`#general`}}](#CHANNEL) as [{{`U1234`}}](#BOT_ID).
```

Create one `.env`-format file per environment:

```sh
# staging.env
CHANNEL=#staging-alerts
BOT_ID=U_STAGING

# prod.env
CHANNEL=#incidents
BOT_ID=U_PROD
```

Build each environment by pointing `--vars` at the right file:

```sh
litprompt build prompt.md --vars staging.env -o dist/staging/prompt.md
litprompt build prompt.md --vars prod.env    -o dist/prod/prompt.md
```

You can **layer** vars files — later files override earlier ones, so a shared base plus an environment overlay is a clean pattern:

```sh
litprompt build prompt.md --vars base.env --vars prod.env -o dist/prod/prompt.md
```

Because **missing variables are a hard error**, you can't accidentally ship a prod build with an unset bot ID — the build fails instead.

## Approach 2: vary targets with `--config`

When environments differ in *which* files build or *where* they go, give each one its own config. Keep the sources identical and change only the `output:` paths so the builds don't clobber each other:

`litprompt.staging.yaml`:

```yaml
builds:
  - source: prompts/
    output: dist/staging/
```

`litprompt.prod.yaml`:

```yaml
builds:
  - source: prompts/
    output: dist/prod/
```

Build each one:

```sh
litprompt build --config litprompt.staging.yaml
litprompt build --config litprompt.prod.yaml
```

Paths in a config resolve **relative to the config file's directory**, so `--config envs/prod.yaml` behaves exactly like `cd envs && litprompt build` pointed at that file. `--config` can't be combined with a source argument, and a missing named config is a hard error.

## Combining both

The usual setup is per-environment configs *and* per-environment vars:

```sh
litprompt build --config litprompt.prod.yaml --vars prod.env
litprompt build --config litprompt.staging.yaml --vars staging.env
```

Wire these up as `make`/`just` targets or CI jobs so an environment build is a single command:

```makefile
prod:    ; litprompt build --config litprompt.prod.yaml --vars prod.env
staging: ; litprompt build --config litprompt.staging.yaml --vars staging.env
```

## Validate an environment before building it

`litprompt check` takes the same config-driven path as `build`, but writes nothing — it just confirms every source resolves and every variable has a value. Use it as a pre-flight check in CI:

```sh
litprompt check --config litprompt.prod.yaml --vars prod.env
```

If `prod.env` is missing a variable that the sources reference, this fails before you ever produce output — answering "will prod build?" without committing to it.

## See also

- [Directive syntax → Variables](../reference/syntax.md#variables) — the precise variable and `.env` rules.
- [Config file reference](../reference/config.md) — config shapes and `--config` resolution.
- [CLI commands](../reference/cli.md) — all flags for `build` and `check`.
