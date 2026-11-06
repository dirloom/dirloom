package cli

import (
	"fmt"
	"io"

	"github.com/dirloom/dirloom/internal/app"
	"github.com/dirloom/dirloom/internal/artifact"
	configuration "github.com/dirloom/dirloom/internal/config"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/presentation"
	"github.com/spf13/cobra"
)

type fingerprintOptions struct {
	root            string
	format          string
	preset          string
	depth           optionalDepth
	directoriesOnly bool
	includeHidden   bool
	ignorePatterns  []string
	noDefaultIgnore bool
	noGitIgnore     bool
	color           string
	icons           string
	theme           string
}

func newFingerprintCommand(stdout io.Writer, loader *configuration.Loader, sources *sourceOptions) *cobra.Command {
	var opts fingerprintOptions
	command := &cobra.Command{
		Use:   "fingerprint [directory]",
		Short: "Fingerprint the structural view produced by Dirloom, not file contents",
		Long: "Compute a deterministic fingerprint of the structural view Dirloom observes\n" +
			"after applying the active filters. The fingerprint does not hash file contents.\n" +
			"Ignore rules, depth, hidden visibility and directories-only change that view\n" +
			"and therefore the fingerprint. Presentation (theme, color, icons, TTY) does not.",
		Example: `  dirloom fingerprint
  dirloom fingerprint --root .
  dirloom fingerprint --format json
  dirloom fingerprint ./src --depth 3`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return &usageError{err: fmt.Errorf("expected at most one directory argument, received %d", len(args))}
			}
			return nil
		},
		ValidArgsFunction: completeInspectRoot,
		RunE: func(cmd *cobra.Command, args []string) error {
			root, err := structuralRoot(cmd, args, opts.root)
			if err != nil {
				return err
			}
			if err := requireEnumeratedFlag("format", opts.format, []string{"text", "json"}, "fingerprint"); err != nil {
				return err
			}
			resolved, _, err := resolveStructuralOptions(cmd, loader, root, *sources, structuralFlagState{
				preset:          opts.preset,
				depth:           opts.depth,
				directoriesOnly: opts.directoriesOnly,
				includeHidden:   opts.includeHidden,
				ignorePatterns:  opts.ignorePatterns,
				noDefaultIgnore: opts.noDefaultIgnore,
				noGitIgnore:     opts.noGitIgnore,
				color:           opts.color,
				icons:           opts.icons,
				theme:           opts.theme,
			})
			if err != nil {
				return err
			}
			result, err := app.Fingerprint(cmd.Context(), app.InspectRequest{
				Root:              resolved.Root,
				MaxDepth:          resolved.Effective.MaxDepth,
				DirectoriesOnly:   resolved.Effective.DirectoriesOnly,
				IncludeHidden:     resolved.Effective.IncludeHidden,
				IgnorePatterns:    resolved.Effective.IgnorePatterns,
				UseDefaultIgnores: resolved.Effective.UseDefaultIgnores,
				UseGitIgnore:      resolved.Effective.UseGitIgnore,
			})
			if err != nil {
				return classifyFingerprintError(err)
			}
			if opts.format == "json" {
				document := identity.NewJSONDocument(result.Fingerprint, result.NodeCount)
				if err := document.WriteJSON(stdout); err != nil {
					return fmt.Errorf("write fingerprint JSON: %w", err)
				}
				return nil
			}
			if err := identity.WriteText(stdout, result.Fingerprint); err != nil {
				return fmt.Errorf("write fingerprint: %w", err)
			}
			return nil
		},
	}
	command.Flags().StringVar(&opts.root, "root", "", "directory to fingerprint (default: positional argument or current directory)")
	command.Flags().StringVar(&opts.format, "format", "text", "output format: text or json")
	command.Flags().StringVar(&opts.preset, "preset", "", "built-in preset: ai, compact, docs, monorepo, or none")
	command.Flags().VarP(&opts.depth, "depth", "d", depthUsage)
	command.Flags().BoolVar(&opts.directoriesOnly, "dirs-only", false, dirsOnlyUsage)
	command.Flags().BoolVar(&opts.includeHidden, "hidden", false, "include hidden entries that survive other filters")
	command.Flags().StringArrayVar(&opts.ignorePatterns, "ignore", nil, "exclude a pattern (repeatable)")
	command.Flags().BoolVar(&opts.noDefaultIgnore, "no-default-ignore", false, "disable built-in directory exclusions")
	command.Flags().BoolVar(&opts.noGitIgnore, "no-gitignore", false, "do not apply .gitignore files")
	// Presentation flags are validated like everywhere else, then ignored:
	// an invalid value is still a usage error (F3).
	command.Flags().StringVar(&opts.color, "color", "", "validated, then ignored: fingerprint identity is presentation-independent")
	command.Flags().StringVar(&opts.icons, "icons", "", "validated, then ignored: fingerprint identity is presentation-independent")
	command.Flags().StringVar(&opts.theme, "theme", "", "validated, then ignored: fingerprint identity is presentation-independent")
	registerFingerprintCompletions(command)
	command.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})
	return command
}

func registerFingerprintCompletions(command *cobra.Command) {
	fixed := func(values []string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return cobra.FixedCompletions(values, cobra.ShellCompDirectiveNoFileComp)
	}
	_ = command.RegisterFlagCompletionFunc("format", fixed([]string{"text", "json"}))
	_ = command.RegisterFlagCompletionFunc("preset", fixed(append(append([]string{}, configuration.PresetNames()...), configuration.PresetNone)))
	_ = command.RegisterFlagCompletionFunc("depth", fixed([]string{"unlimited"}))
	_ = command.RegisterFlagCompletionFunc("color", fixed(presentation.ColorModes()))
	_ = command.RegisterFlagCompletionFunc("icons", fixed(presentation.IconModes()))
	_ = command.RegisterFlagCompletionFunc("theme", completeTheme)
	_ = command.RegisterFlagCompletionFunc("root", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveFilterDirs
	})
}

func classifyFingerprintError(err error) error {
	if configuration.IsInvalid(err) {
		return &usageError{err: err}
	}
	if artifact.IsInternal(err) {
		return fmt.Errorf("internal error: %w", err)
	}
	return err
}
