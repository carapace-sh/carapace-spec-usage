// Package usage converts a usage spec (https://usage.jdx.dev) into a
// carapace-spec command tree.
//
// The input is the JSON form of a usage spec, as produced by
// `usage generate json` (or by any tool that serializes its usage spec with
// serde). The output is a github.com/carapace-sh/carapace-spec command.Command
// which marshals to carapace YAML.
package usage

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/carapace-sh/carapace-spec/pkg/command"
)

// Spec is the root of a usage spec.
type Spec struct {
	Name     string                  `json:"name"`
	Bin      string                  `json:"bin"`
	Version  *string                 `json:"version"`
	About    *string                 `json:"about"`
	Cmd      SpecCommand             `json:"cmd"`
	Complete map[string]SpecComplete `json:"complete"`
}

// SpecCommand is a command in a usage spec.
type SpecCommand struct {
	Name          string                  `json:"name"`
	Help          *string                 `json:"help"`
	HelpLong      *string                 `json:"help_long"`
	HelpHeading   *string                 `json:"help_heading"`
	Aliases       []string                `json:"aliases"`
	HiddenAliases []string                `json:"hidden_aliases"`
	Hide          bool                    `json:"hide"`
	Deprecated    *string                 `json:"deprecated"`
	Flags         []SpecFlag              `json:"flags"`
	Args          []SpecArg               `json:"args"`
	Groups        []SpecGroup             `json:"groups"`
	Subcommands   Subcommands             `json:"subcommands"`
	Examples      []SpecExample           `json:"examples"`
	Complete      map[string]SpecComplete `json:"complete"`
}

// SpecFlag is a flag in a usage spec.
type SpecFlag struct {
	Name               string   `json:"name"`
	Help               *string  `json:"help"`
	HelpLong           *string  `json:"help_long"`
	Short              []string `json:"short"`
	HiddenShortAliases []string `json:"hidden_short_aliases"`
	Long               []string `json:"long"`
	HiddenAliases      []string `json:"hidden_aliases"`
	Required           bool     `json:"required"`
	Var                bool     `json:"var"`
	Count              bool     `json:"count"`
	Hide               bool     `json:"hide"`
	Global             bool     `json:"global"`
	Arg                *SpecArg `json:"arg"`
	Default            []string `json:"default"`
	Negate             *string  `json:"negate"`
	Deprecated         *string  `json:"deprecated"`
	Conflicts          []string `json:"conflicts"`
}

// SpecArg is an argument (a positional or a flag's value) in a usage spec.
type SpecArg struct {
	Name       string        `json:"name"`
	ValueNames []string      `json:"value_names"`
	Help       *string       `json:"help"`
	HelpLong   *string       `json:"help_long"`
	Required   bool          `json:"required"`
	Var        bool          `json:"var"`
	VarMax     *int          `json:"var_max"`
	Hide       bool          `json:"hide"`
	Default    []string      `json:"default"`
	Choices    *SpecChoices  `json:"choices"`
	Complete   *SpecComplete `json:"complete"`
}

// SpecComplete is a completion declaration in a usage spec.
type SpecComplete struct {
	Run          *string `json:"run"`
	Descriptions bool    `json:"descriptions"`
	Type         *string `json:"type_"`
	Delegate     *string `json:"delegate"`
}

// SpecChoices is a set of valid values for an argument.
type SpecChoices struct {
	Choices    []string     `json:"choices"`
	Details    []SpecChoice `json:"details"`
	IgnoreCase bool         `json:"ignore_case"`
	Run        *string      `json:"run"`
}

// SpecChoice is metadata for a single choice.
type SpecChoice struct {
	Value   string            `json:"value"`
	Help    *string           `json:"help"`
	Hide    bool              `json:"hide"`
	Aliases []SpecChoiceAlias `json:"aliases"`
}

// SpecChoiceAlias is an alternative spelling for a choice.
type SpecChoiceAlias struct {
	Value string `json:"value"`
	Hide  bool   `json:"hide"`
}

// SpecGroup is a set of related flags.
type SpecGroup struct {
	Name     string   `json:"name"`
	Members  []string `json:"members"`
	Required bool     `json:"required"`
	Multiple bool     `json:"multiple"`
}

// SpecExample is an example invocation in a usage spec.
type SpecExample struct {
	Code   string  `json:"code"`
	Header *string `json:"header"`
	Help   *string `json:"help"`
	Lang   string  `json:"lang"`
}

// Subcommands is an ordered map of subcommands, preserving the order the spec
// declares them in.
type Subcommands struct {
	keys   []string
	values map[string]SpecCommand
}

// UnmarshalJSON decodes the object while recording key order, which a plain
// map would lose.
func (s *Subcommands) UnmarshalJSON(data []byte) error {
	s.values = make(map[string]SpecCommand)

	dec := json.NewDecoder(bytes.NewReader(data))
	token, err := dec.Token()
	if err != nil {
		return err
	}
	if delim, ok := token.(json.Delim); !ok || delim != '{' {
		return fmt.Errorf("expected subcommands to be an object")
	}

	for dec.More() {
		token, err := dec.Token()
		if err != nil {
			return err
		}
		key := token.(string)

		var sub SpecCommand
		if err := dec.Decode(&sub); err != nil {
			return err
		}

		s.keys = append(s.keys, key)
		s.values[key] = sub
	}
	return nil
}

// Command converts a usage spec (JSON) into a carapace-spec command tree.
func Command(data []byte) (command.Command, error) {
	var spec Spec
	if err := json.Unmarshal(data, &spec); err != nil {
		return command.Command{}, fmt.Errorf("invalid usage spec: %w", err)
	}

	c := &converter{spec: &spec}
	cmd := c.convertCommand(spec.Name, &spec.Cmd)
	if cmd.Description == "" && spec.About != nil {
		cmd.Description = *spec.About
	}
	return cmd, nil
}

type converter struct {
	spec *Spec
}

func (c *converter) convertCommand(name string, sc *SpecCommand) command.Command {
	cmd := command.Command{
		Name:        name,
		Description: withDeprecation(firstString(sc.Help, sc.HelpLong), sc.Deprecated),
		Aliases:     merge(sc.Aliases, sc.HiddenAliases),
		Hidden:      sc.Hide,
	}
	if sc.HelpHeading != nil {
		cmd.Group = *sc.HelpHeading
	}
	if sc.HelpLong != nil {
		cmd.Documentation.Command = *sc.HelpLong
	}

	for i := range sc.Flags {
		flag := &sc.Flags[i]
		for _, f := range c.convertFlag(flag) {
			cmd.AddFlag(f)
		}
		if values := c.flagCompletion(sc, flag); len(values) > 0 {
			if cmd.Completion.Flag == nil {
				cmd.Completion.Flag = make(map[string][]string)
			}
			cmd.Completion.Flag[flagKey(flag)] = values
		}
		if flag.HelpLong != nil {
			if cmd.Documentation.Flag == nil {
				cmd.Documentation.Flag = make(map[string]string)
			}
			cmd.Documentation.Flag[flagKey(flag)] = *flag.HelpLong
		}
	}

	c.convertArgs(&cmd, sc)
	cmd.ExclusiveFlags = c.exclusiveFlags(sc)

	for _, key := range sc.Subcommands.keys {
		sub := sc.Subcommands.values[key]
		cmd.Commands = append(cmd.Commands, c.convertCommand(key, &sub))
	}

	cmd.Examples = examples(sc.Examples)
	return cmd
}

// convertFlag returns the carapace flags for a usage flag. carapace expresses a
// single long and a single short per entry, so a flag with aliases becomes
// several entries: the primary form keeps the flag's own visibility, aliases
// keep their own (hidden unless they are visible aliases), and a negate prefix
// becomes an additional hidden boolean flag.
func (c *converter) convertFlag(f *SpecFlag) []command.Flag {
	base := command.Flag{
		Description: withDeprecation(firstString(f.Help, f.HelpLong), f.Deprecated),
		Value:       f.Arg != nil,
		Repeatable:  f.Var || f.Count,
		Required:    f.Required,
		Hidden:      f.Hide,
		Persistent:  f.Global,
		Default:     strings.Join(f.Default, ","),
	}
	if f.Arg != nil {
		if !f.Arg.Required {
			base.Optarg = true
		}
		if n := len(f.Arg.ValueNames); n > 1 {
			base.Nargs = n
		}
	}

	shorts := unique(append(append([]string{}, f.Short...), f.HiddenShortAliases...))
	longs := unique(append(append([]string{}, f.Long...), f.HiddenAliases...))
	hiddenShorts := set(f.HiddenShortAliases)
	hiddenLongs := set(f.HiddenAliases)

	var flags []command.Flag
	if len(shorts) == 0 && len(longs) == 0 {
		return nil
	}

	var primaryShort, primaryLong string
	if len(shorts) > 0 {
		primaryShort = shorts[0]
		shorts = shorts[1:]
	}
	if len(longs) > 0 {
		primaryLong = longs[0]
		longs = longs[1:]
	}
	flags = append(flags, newFlag(base, primaryShort, primaryLong, f.Hide))

	for _, short := range shorts {
		flags = append(flags, newFlag(base, short, "", f.Hide || hiddenShorts[short]))
	}
	for _, long := range longs {
		flags = append(flags, newFlag(base, "", long, f.Hide || hiddenLongs[long]))
	}

	if f.Negate != nil {
		if name := strings.TrimLeft(*f.Negate, "-"); name != "" {
			negate := base
			negate.Longhand = name
			negate.Shorthand = ""
			negate.Value = false
			negate.Optarg = false
			negate.Repeatable = false
			negate.Required = false
			negate.Hidden = true
			negate.Default = ""
			negate.Nargs = 0
			flags = append(flags, negate)
		}
	}

	return flags
}

func newFlag(base command.Flag, short, long string, hidden bool) command.Flag {
	flag := base
	flag.Shorthand = short
	flag.Longhand = long
	flag.Hidden = hidden
	return flag
}

// convertArgs converts positional arguments. carapace only knows positionals
// through their completions, so each argument becomes one entry in
// Completion.Positional (or PositionalAny when variadic). Documentation is
// aligned with the same list.
func (c *converter) convertArgs(cmd *command.Command, sc *SpecCommand) {
	var (
		positional    [][]string
		positionalDoc []string
	)
	for i := range sc.Args {
		arg := &sc.Args[i]
		if arg.Var {
			if values := c.argCompletion(sc, arg); len(values) > 0 {
				cmd.Completion.PositionalAny = values
			}
			if doc := argDescription(arg); doc != "" {
				cmd.Documentation.PositionalAny = doc
			}
			continue
		}
		positional = append(positional, c.argCompletion(sc, arg))
		positionalDoc = append(positionalDoc, argDescription(arg))
	}

	if len(positional) > 0 {
		cmd.Completion.Positional = positional
		cmd.Documentation.Positional = positionalDoc
	}
}

// flagCompletion resolves the completion values for a flag's value.
func (c *converter) flagCompletion(sc *SpecCommand, f *SpecFlag) []string {
	if f.Arg == nil {
		return nil
	}
	return c.argCompletion(sc, f.Arg)
}

// argCompletion assembles the carapace completion values for an argument from
// its choices and completer.
func (c *converter) argCompletion(sc *SpecCommand, arg *SpecArg) []string {
	var values []string

	if arg.Choices != nil {
		values = append(values, choiceValues(arg.Choices)...)
	}
	if complete := c.completer(sc, arg); complete != nil {
		if macro := completeMacro(complete); macro != "" {
			values = append(values, macro)
		}
	}
	return values
}

// completer resolves the completer for an argument the way usage does: a
// `complete` written inside the argument wins, then one naming the argument at
// the top level of the spec, then one on the command.
func (c *converter) completer(sc *SpecCommand, arg *SpecArg) *SpecComplete {
	if arg.Complete != nil {
		return arg.Complete
	}
	name := strings.ToLower(arg.Name)
	if complete, ok := c.spec.Complete[name]; ok {
		return &complete
	}
	if complete, ok := sc.Complete[name]; ok {
		return &complete
	}
	return nil
}

// completeMacro maps a usage completer to a carapace macro. Custom `run`
// scripts that use usage's templating ({{words[...]}}) are skipped rather than
// emitted with placeholders carapace cannot resolve.
func completeMacro(complete *SpecComplete) string {
	if complete.Type != nil {
		if macro := typeMacros[*complete.Type]; macro != "" {
			return macro
		}
	}
	if complete.Run != nil && !strings.Contains(*complete.Run, "{{") {
		return "$sh(" + *complete.Run + ")"
	}
	return ""
}

// typeMacros maps usage's built-in completion types to carapace macros.
var typeMacros = map[string]string{
	"dir":  "$directories",
	"file": "$files",
	"path": "$files",
	"url":  "$carapace.net.http.Urls",
}

// choiceValues renders usage choices as carapace values, including aliases and
// tab-separated descriptions.
func choiceValues(choices *SpecChoices) []string {
	var values []string

	details := make(map[string]SpecChoice, len(choices.Details))
	for _, detail := range choices.Details {
		details[detail.Value] = detail
	}

	for _, value := range choices.Choices {
		values = append(values, choiceValue(value, details[value]))
	}
	for _, detail := range choices.Details {
		for _, alias := range detail.Aliases {
			values = append(values, choiceValue(alias.Value, SpecChoice{}))
		}
	}

	if choices.Run != nil && !strings.Contains(*choices.Run, "{{") {
		values = append(values, "$sh("+*choices.Run+")")
	}
	return values
}

func choiceValue(value string, detail SpecChoice) string {
	if detail.Help != nil && *detail.Help != "" {
		return value + "\t" + *detail.Help
	}
	return value
}

// exclusiveFlags converts usage groups and conflicts into carapace mutually
// exclusive flag groups. A group that allows more than one member is not
// exclusive and is skipped. Conflicts are only emitted when both members are
// defined on the same command, since carapace requires it.
func (c *converter) exclusiveFlags(sc *SpecCommand) [][]string {
	var result [][]string

	defined := make(map[string]bool)
	for i := range sc.Flags {
		defined[flagKey(&sc.Flags[i])] = true
	}

	for _, group := range sc.Groups {
		if group.Multiple || len(group.Members) < 2 {
			continue
		}
		var members []string
		for _, member := range group.Members {
			members = append(members, flagName(member))
		}
		result = append(result, members)
	}

	for i := range sc.Flags {
		flag := &sc.Flags[i]
		for _, conflict := range flag.Conflicts {
			name := flagName(conflict)
			if !defined[name] || !defined[flagKey(flag)] {
				continue
			}
			result = append(result, []string{flagKey(flag), name})
		}
	}

	return result
}

func flagKey(f *SpecFlag) string {
	if len(f.Long) > 0 {
		return f.Long[0]
	}
	if len(f.Short) > 0 {
		return f.Short[0]
	}
	return f.Name
}

func flagName(selector string) string {
	return strings.TrimLeft(selector, "-")
}

func argDescription(arg *SpecArg) string {
	return firstString(arg.Help, arg.HelpLong)
}

func examples(examples []SpecExample) map[string]string {
	if len(examples) == 0 {
		return nil
	}

	result := make(map[string]string, len(examples))
	for _, example := range examples {
		if example.Help != nil {
			result[example.Code] = *example.Help
		}
	}
	if len(result) == 0 {
		return nil
	}
	return result
}

func withDeprecation(description string, deprecated *string) string {
	if deprecated == nil || *deprecated == "" {
		return description
	}
	if description == "" {
		return "(deprecated: " + *deprecated + ")"
	}
	return description + " (deprecated: " + *deprecated + ")"
}

func firstString(values ...*string) string {
	for _, value := range values {
		if value != nil && *value != "" {
			return *value
		}
	}
	return ""
}

func merge(a, b []string) []string {
	return append(append([]string{}, a...), b...)
}

func unique(values []string) []string {
	seen := make(map[string]bool, len(values))
	var result []string
	for _, value := range values {
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func set(values []string) map[string]bool {
	result := make(map[string]bool, len(values))
	for _, value := range values {
		result[value] = true
	}
	return result
}
