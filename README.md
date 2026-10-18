# carapace-spec-usage

[Spec](https://github.com/carapace-sh/carapace-spec) generation from a [usage](https://usage.jdx.dev) spec.

Converts the JSON form of a usage spec (as produced by `usage generate json`) into carapace-spec YAML for [carapace](https://carapace.sh) shell completion.

## How it works

```
usage spec (KDL)
  → usage generate json
    → usage spec (JSON)
      → carapace-spec-usage (Go)
        → carapace-spec YAML
```

## Usage

```bash
# convert a file
carapace-spec-usage mise.json > mise.yaml

# or read from stdin
mise usage | usage generate json | carapace-spec-usage > mise.yaml
```

The output is written to stdout, prefixed with the carapace schema header.

## Library

```go
import (
	"os"

	"github.com/carapace-sh/carapace-spec-usage/pkg/usage"
	"gopkg.in/yaml.v3"
)

data, _ := os.ReadFile("mise.json")
cmd, err := usage.Command(data)
if err != nil {
	panic(err)
}
out, _ := yaml.Marshal(cmd)
```

`usage.Command` returns a `github.com/carapace-sh/carapace-spec/pkg/command.Command`.

## What is converted

- **Commands**: name, description, aliases, hidden, help heading (`group`) and
  examples; long help goes to `documentation.command`.
- **Flags**: value/optional-value flags, repeatable and count flags, persistent
  globals, required, hidden, defaults (extended notation), negation flags, and
  visible versus hidden long/short aliases (each alias becomes its own entry).
  Long help goes to `documentation.flag`.
- **Positional arguments**: mapped to `completion.positional` /
  `positionalany` (carapace only knows positionals through their completions)
  with aligned `documentation.positional`.
- **Completions**: `choices` (including per-choice help and aliases), inline and
  named `complete` nodes, and built-in `type_` values (`dir`/`file`/`path`/`url`).
- **Exclusive flags**: usage `groups` (when mutually exclusive) and same-command
  `conflicts`.

Not converted (no clean carapace equivalent): `overrides`, `default_subcommand`,
`mounts`, `views`, `outputs`, `effect`, `exit_codes`, and the requiredness
conditions (`required_if`, `required_unless`).

`complete`/`choices` `run` scripts that use usage's templating (`{{words[...]}}`)
are skipped rather than emitted with placeholders carapace cannot resolve;
static scripts become `$sh(...)`.
