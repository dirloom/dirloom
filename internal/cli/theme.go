package cli

import (
	"bytes"
	"fmt"
	"io"

	configuration "github.com/dirloom/dirloom/internal/config"
	"github.com/dirloom/dirloom/internal/presentation"
	"github.com/dirloom/dirloom/internal/render"
	"github.com/spf13/cobra"
)

func rejectSourceFlags(command *cobra.Command, sources *sourceOptions, context string) error {
	checks := []struct {
		name    string
		changed bool
	}{
		{name: "config", changed: command.Flags().Changed("config") || sources.path != ""},
		{name: "no-user-config", changed: command.Flags().Changed("no-user-config") || sources.noUser},
		{name: "no-config", changed: command.Flags().Changed("no-config") || sources.noConfig},
	}
	for _, check := range checks {
		if check.changed {
			return &usageError{err: fmt.Errorf("--%s cannot be used with %s", check.name, context)}
		}
	}
	return nil
}

func validateFormatOptions(resolution configuration.Resolution, overrides configuration.Overrides) error {
	if resolution.Effective.Format == render.FormatJSON && overrides.Style.Set {
		return &usageError{err: fmt.Errorf("--style cannot be used with --format json")}
	}
	if resolution.Effective.Format == render.FormatText {
		return nil
	}
	if overrides.Color.Set && overrides.Color.Value != presentation.ColorNever {
		return &usageError{err: fmt.Errorf("--color %s cannot be used with --format %s; use --color never for a canonical artifact", overrides.Color.Value, resolution.Effective.Format)}
	}
	if overrides.Icons.Set && overrides.Icons.Value != presentation.IconsNever {
		return &usageError{err: fmt.Errorf("--icons %s cannot be used with --format %s; use --icons never for a canonical artifact", overrides.Icons.Value, resolution.Effective.Format)}
	}
	if overrides.Theme.Set {
		return &usageError{err: fmt.Errorf("--theme cannot be used with --format %s", resolution.Effective.Format)}
	}
	return nil
}

func loadEffectiveTheme(resolution configuration.Resolution) (presentation.Theme, *presentation.CompiledTheme, error) {
	origin := resolution.Provenance["theme"]
	context := presentation.ReferenceContext{Kind: string(origin.Source), ConfigPath: origin.Path}
	if origin.Source == configuration.SourceCLI || origin.Source == configuration.SourceBuiltIn {
		context.Kind = "cli"
	}
	theme, err := presentation.LoadReference(resolution.Effective.Theme, context)
	if err != nil {
		return presentation.Theme{}, nil, err
	}
	compiled, err := presentation.Compile(theme)
	if err != nil {
		return presentation.Theme{}, nil, err
	}
	return theme, compiled, nil
}

func classifyPresentationError(err error) error {
	if presentation.IsInvalid(err) {
		return &usageError{err: err}
	}
	return err
}

func newThemeCommand(stdout io.Writer, sources *sourceOptions) *cobra.Command {
	command := &cobra.Command{
		Use: "theme", Short: "Inspect and validate terminal themes",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 {
				return &usageError{err: fmt.Errorf("expected a theme subcommand, received %d argument(s)", len(args))}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := rejectSourceFlags(cmd, sources, "theme inspection"); err != nil {
				return err
			}
			return cmd.Help()
		},
	}
	command.AddCommand(newThemeListCommand(stdout, sources))
	command.AddCommand(newThemeExplainCommand(stdout, sources))
	command.AddCommand(newThemeValidateCommand(stdout, sources))
	return command
}

func newThemeListCommand(stdout io.Writer, sources *sourceOptions) *cobra.Command {
	var as string
	command := &cobra.Command{
		Use: "list", Short: "List built-in themes",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 {
				return &usageError{err: fmt.Errorf("expected no arguments, received %d", len(args))}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := rejectSourceFlags(cmd, sources, "theme inspection"); err != nil {
				return err
			}
			return writeThemeResult(stdout, as, presentation.WriteListText, presentation.WriteListJSON, "theme list")
		},
	}
	command.Flags().StringVar(&as, "as", "text", "output format: text or json")
	return command
}

func newThemeExplainCommand(stdout io.Writer, sources *sourceOptions) *cobra.Command {
	var as string
	command := &cobra.Command{
		Use: "explain <theme>", Short: "Explain a built-in or local YAML theme",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return &usageError{err: fmt.Errorf("expected exactly one theme name or path, received %d", len(args))}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rejectSourceFlags(cmd, sources, "theme inspection"); err != nil {
				return err
			}
			if as != "text" && as != "json" {
				return &usageError{err: fmt.Errorf("unsupported output format %q (expected text or json)", as)}
			}
			theme, err := presentation.LoadReference(args[0], presentation.ReferenceContext{Kind: "cli"})
			if err != nil {
				return classifyPresentationError(err)
			}
			return writeThemeResult(stdout, as, theme.WriteText, theme.WriteJSON, "theme explanation")
		},
	}
	command.Flags().StringVar(&as, "as", "text", "output format: text or json")
	return command
}

func newThemeValidateCommand(stdout io.Writer, sources *sourceOptions) *cobra.Command {
	var as string
	command := &cobra.Command{
		Use: "validate <path>", Short: "Validate a local YAML theme without scanning",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return &usageError{err: fmt.Errorf("expected exactly one theme path, received %d", len(args))}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rejectSourceFlags(cmd, sources, "theme inspection"); err != nil {
				return err
			}
			if as != "text" && as != "json" {
				return &usageError{err: fmt.Errorf("unsupported output format %q (expected text or json)", as)}
			}
			if !presentation.IsThemePath(args[0]) {
				return &usageError{err: fmt.Errorf("theme validate requires a .yaml/.yml path")}
			}
			theme, err := presentation.LoadReference(args[0], presentation.ReferenceContext{Kind: "cli"})
			if err != nil {
				return classifyPresentationError(err)
			}
			result := presentation.Validation(theme)
			return writeThemeResult(stdout, as, result.WriteText, result.WriteJSON, "theme validation")
		},
	}
	command.Flags().StringVar(&as, "as", "text", "output format: text or json")
	return command
}

func writeThemeResult(stdout io.Writer, as string, textWriter, jsonWriter func(io.Writer) error, context string) error {
	var buffer bytes.Buffer
	switch as {
	case "text":
		if err := textWriter(&buffer); err != nil {
			return fmt.Errorf("write %s: %w", context, err)
		}
	case "json":
		if err := jsonWriter(&buffer); err != nil {
			return fmt.Errorf("write %s: %w", context, err)
		}
	default:
		return &usageError{err: fmt.Errorf("unsupported output format %q (expected text or json)", as)}
	}
	if err := writeAll(stdout, buffer.Bytes()); err != nil {
		return fmt.Errorf("write %s: %w", context, err)
	}
	return nil
}
