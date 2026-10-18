package usage

import (
	"flag"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/carapace-sh/carapace-spec/pkg/command"
	"gopkg.in/yaml.v3"
)

var update = flag.Bool("update", false, "update golden files")

func TestCommand(t *testing.T) {
	entries, err := filepath.Glob("testdata/*.json")
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) == 0 {
		t.Fatal("no testdata found")
	}

	for _, entry := range entries {
		name := strings.TrimSuffix(filepath.Base(entry), ".json")
		t.Run(name, func(t *testing.T) {
			data, err := os.ReadFile(entry)
			if err != nil {
				t.Fatal(err)
			}

			converted, err := Command(data)
			if err != nil {
				t.Fatal(err)
			}

			got, err := yaml.Marshal(converted)
			if err != nil {
				t.Fatal(err)
			}

			golden := strings.TrimSuffix(entry, ".json") + ".yaml"
			if *update {
				if err := os.WriteFile(golden, got, 0o644); err != nil {
					t.Fatal(err)
				}
				return
			}

			want, err := os.ReadFile(golden)
			if err != nil {
				t.Fatal(err)
			}

			if string(got) != string(want) {
				t.Errorf("unexpected spec\n--- got ---\n%s\n--- want ---\n%s", got, want)
			}

			var roundTrip command.Command
			if err := yaml.Unmarshal(want, &roundTrip); err != nil {
				t.Errorf("generated spec does not parse as a carapace spec: %v", err)
			}
		})
	}
}

func TestCommandInvalid(t *testing.T) {
	if _, err := Command([]byte("not json")); err == nil {
		t.Error("expected an error for invalid input")
	}
}
