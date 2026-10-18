package main

import (
	"fmt"
	"io"
	"os"

	"github.com/carapace-sh/carapace-spec-usage/pkg/usage"
	"github.com/spf13/cobra"
	"gopkg.in/yaml.v3"
)

func main() {
	cmd := &cobra.Command{
		Use:   "carapace-spec-usage [spec.json]",
		Short: "Convert a usage spec to a carapace-spec",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			var (
				data []byte
				err  error
			)
			if len(args) > 0 {
				data, err = os.ReadFile(args[0])
			} else {
				data, err = io.ReadAll(cmd.InOrStdin())
			}
			if err != nil {
				return err
			}

			converted, err := usage.Command(data)
			if err != nil {
				return err
			}

			out, err := yaml.Marshal(converted)
			if err != nil {
				return err
			}

			fmt.Println("# yaml-language-server: $schema=https://carapace.sh/schemas/command.json")
			fmt.Print(string(out))
			return nil
		},
	}

	if err := cmd.Execute(); err != nil {
		os.Exit(1)
	}
}
