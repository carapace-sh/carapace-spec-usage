# AGENTS.md

Guide for AI agents working in this repository.

## What this is

A Go tool that generates [carapace-spec](https://github.com/carapace-sh/carapace-spec)
YAML from the JSON form of a [usage](https://usage.jdx.dev) spec.

The library lives in `pkg/usage` (package `usage`) and exposes a single function,
`Command(data []byte) (command.Command, error)`. The CLI in
`cmd/carapace-spec-usage` reads a JSON file (or stdin) and prints the YAML to
stdout with the carapace schema header.

## Commands

All commands run from the repo root. Module requires Go 1.24 (see `go.mod`).

- `go build -v ./...` — build
- `go test -v -coverprofile=profile.cov ./...` — test
- `go test ./pkg/usage -update` — regenerate golden files in `pkg/usage/testdata`
- `go vet ./...` — vet
- `gofmt -d -s .` — format check (CI fails if this prints anything; fix with `gofmt -w -s .`)
- `staticcheck ./...` — static analysis

CI (`.github/workflows/go.yml`) runs: build → test → format check → coverage → staticcheck.

## Architecture & data flow

```
usage spec JSON
   │  json.Unmarshal into Spec / SpecCommand / SpecFlag / SpecArg / SpecComplete / SpecChoices
   ▼
(*converter).convertCommand → convertFlag / convertArgs / argCompletion / exclusiveFlags
   ▼
command.Command  (github.com/carapace-sh/carapace-spec/pkg/command)
   │  yaml.Marshal
   ▼
carapace YAML
```

- `Command` decodes the JSON and builds a `converter` (which holds the spec's
  top-level `complete` map for name lookups), then calls `convertCommand`.
- `convertCommand` maps one command: name, description, aliases, hidden, group
  (`help_heading`), flags, positional args, exclusive flags, subcommands and
  examples.
- `convertFlag` returns `[]command.Flag`. carapace expresses only one long and one
  short per flag, so a usage flag with extra `long`/`short` aliases becomes
  several flags: the primary form visible, the remaining forms hidden only if
  they appear in `hidden_aliases`/`hidden_short_aliases`. A `negate` becomes an
  additional hidden boolean flag.
- `convertArgs` maps positional args to `Completion.Positional` (and
  `Completion.PositionalAny` for a variadic), with aligned
  `Documentation.Positional`. carapace only knows positionals through their
  completions, so args without one still occupy a slot (an empty action).
- `argCompletion` combines `choices` and the resolved `complete` node.
  `completer` mirrors usage's lookup order: inline `complete`, then the spec's
  top-level map, then the command's map, each keyed by the lowercased arg name.
- `exclusiveFlags` converts mutually exclusive `groups` and same-command
  `conflicts` into `ExclusiveFlags`. Members are stored bare (no leading dashes).

## Conventions

- `Subcommands` is an ordered map with a custom `UnmarshalJSON` so declaration
  order is preserved (a plain `map` would lose it).
- `command.Flag.Longhand`/`Shorthand` store bare names; `Flag.format()` adds the
  dashes. Never include the leading dashes in those fields.
- Persistent flags come from a flag's `global` field, which in a usage spec only
  appears on the root command.
- The completion/doc key for a flag is its primary long name, falling back to the
  primary short (`flagKey`).
- `complete`/`choices` `run` scripts containing `{{...}}` templating are skipped;
  static scripts become `$sh(...)`. Built-in `type_` values map in `typeMacros`.
- The schema header is added by the CLI, not the library, matching
  `carapace-spec-argparse`.

## Scope

Converted: commands, flags (incl. aliases/negation/defaults/persistence),
positionals, completions (choices + `complete`), exclusive flags, examples,
documentation and nargs. Not converted because carapace cannot express them:
`overrides`, `default_subcommand`, `mounts`, `views`, `outputs`, `effect`,
`exit_codes`, `required_if`/`required_unless`. Do not add KDL parsing: the JSON
form produced by `usage generate json` is the supported input.
