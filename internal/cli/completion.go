package cli

import (
	"fmt"
	"io"
	"strings"

	configuration "github.com/dirloom/dirloom/internal/config"
	"github.com/dirloom/dirloom/internal/diagram"
	"github.com/dirloom/dirloom/internal/outputformat"
	"github.com/dirloom/dirloom/internal/presentation"
	"github.com/spf13/cobra"
)

var completionShells = []string{"bash", "zsh", "fish", "powershell"}

func newCompletionCommand(stdout io.Writer) *cobra.Command {
	var noDescriptions bool
	command := &cobra.Command{
		Use:   "completion [bash|zsh|fish|powershell]",
		Short: "Generate shell completion scripts",
		Long: "Generate a completion script for the requested shell. The script is written to stdout\n" +
			"and does not modify any shell profile. Source or install it with your shell's usual mechanism.",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return &usageError{err: fmt.Errorf("expected exactly one shell name, received %d", len(args))}
			}
			return nil
		},
		ValidArgs: completionShells,
		RunE: func(cmd *cobra.Command, args []string) error {
			shell := args[0]
			root := cmd.Root()
			var buffer strings.Builder
			var err error
			switch shell {
			case "bash":
				err = root.GenBashCompletionV2(&buffer, !noDescriptions)
			case "zsh":
				if noDescriptions {
					err = root.GenZshCompletionNoDesc(&buffer)
				} else {
					err = root.GenZshCompletion(&buffer)
				}
			case "fish":
				err = root.GenFishCompletion(&buffer, !noDescriptions)
			case "powershell":
				if noDescriptions {
					err = root.GenPowerShellCompletion(&buffer)
				} else {
					err = root.GenPowerShellCompletionWithDesc(&buffer)
				}
			default:
				return &usageError{err: fmt.Errorf("unsupported shell %q (expected %s)", shell, strings.Join(completionShells, ", "))}
			}
			if err != nil {
				return fmt.Errorf("generate %s completion: %w", shell, err)
			}
			script := buffer.String()
			if script != "" && !strings.HasSuffix(script, "\n") {
				script += "\n"
			}
			if err := writeAll(stdout, []byte(script)); err != nil {
				return fmt.Errorf("write completion script: %w", err)
			}
			return nil
		},
	}
	command.Flags().BoolVar(&noDescriptions, "no-descriptions", false, "disable completion descriptions")
	command.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})
	return command
}

func registerInspectCompletions(command *cobra.Command) {
	fixed := func(values []string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return cobra.FixedCompletions(values, cobra.ShellCompDirectiveNoFileComp)
	}
	_ = command.RegisterFlagCompletionFunc("format", fixed(outputformat.AcceptedNames()))
	_ = command.RegisterFlagCompletionFunc("preset", fixed(append(append([]string{}, configuration.PresetNames()...), configuration.PresetNone)))
	_ = command.RegisterFlagCompletionFunc("style", fixed([]string{"unicode", "ascii"}))
	_ = command.RegisterFlagCompletionFunc("color", fixed(presentation.ColorModes()))
	_ = command.RegisterFlagCompletionFunc("icons", fixed(presentation.IconModes()))
	_ = command.RegisterFlagCompletionFunc("theme", completeTheme)
	_ = command.RegisterFlagCompletionFunc("depth", fixed([]string{"unlimited"}))
	_ = command.RegisterFlagCompletionFunc("diagram-view", fixed([]string{string(diagram.ViewStructure)}))
	_ = command.RegisterFlagCompletionFunc("diagram-direction", fixed([]string{string(diagram.DirectionTopDown), string(diagram.DirectionLeftRight)}))
	_ = command.RegisterFlagCompletionFunc("diagram-max-nodes", fixed([]string{"unlimited"}))
}

func completeInspectRoot(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	if len(args) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	return nil, cobra.ShellCompDirectiveFilterDirs
}

func completeTheme(_ *cobra.Command, _ []string, _ string) ([]string, cobra.ShellCompDirective) {
	return presentation.ThemeNames(), cobra.ShellCompDirectiveDefault
}

func registerAsCompletions(command *cobra.Command) {
	_ = command.RegisterFlagCompletionFunc("as", cobra.FixedCompletions([]string{"text", "json"}, cobra.ShellCompDirectiveNoFileComp))
}
