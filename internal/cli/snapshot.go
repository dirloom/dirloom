package cli

import (
	"fmt"
	"io"

	"github.com/dirloom/dirloom/internal/app"
	"github.com/dirloom/dirloom/internal/artifact"
	configuration "github.com/dirloom/dirloom/internal/config"
	"github.com/dirloom/dirloom/internal/output"
	"github.com/dirloom/dirloom/internal/presentation"
	"github.com/dirloom/dirloom/internal/snapshot"
	"github.com/spf13/cobra"
)

type snapshotOptions struct {
	root            string
	output          string
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

func newSnapshotCommand(stdout io.Writer, loader *configuration.Loader, sources *sourceOptions) *cobra.Command {
	var opts snapshotOptions
	command := &cobra.Command{
		Use:   "snapshot [directory]",
		Short: "Persist a self-verifying structural snapshot",
		Long: "Capture the structural view Dirloom observes after applying the active\n" +
			"filters into Snapshot Schema v1 JSON. The embedded fingerprint is Fingerprint\n" +
			"v1 of that artifact; it does not hash file contents or the snapshot JSON bytes.\n" +
			"Presentation (theme, color, icons, TTY) does not enter the snapshot.",
		Example: `  dirloom snapshot
  dirloom snapshot .
  dirloom snapshot --root .
  dirloom snapshot --output architecture.dlm.json`,
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
			result, err := app.Snapshot(cmd.Context(), app.InspectRequest{
				Root:              resolved.Root,
				MaxDepth:          resolved.Effective.MaxDepth,
				DirectoriesOnly:   resolved.Effective.DirectoriesOnly,
				IncludeHidden:     resolved.Effective.IncludeHidden,
				IgnorePatterns:    resolved.Effective.IgnorePatterns,
				UseDefaultIgnores: resolved.Effective.UseDefaultIgnores,
				UseGitIgnore:      resolved.Effective.UseGitIgnore,
				OutputPath:        opts.output,
			})
			if err != nil {
				return classifySnapshotError(err)
			}
			if opts.output != "" {
				if err := output.WriteFile(opts.output, result.Bytes); err != nil {
					return fmt.Errorf("write snapshot: %w", err)
				}
				return nil
			}
			if err := writeAll(stdout, result.Bytes); err != nil {
				return fmt.Errorf("write snapshot: %w", err)
			}
			return nil
		},
	}
	command.Flags().StringVar(&opts.root, "root", "", "directory to snapshot (default: positional argument or current directory)")
	command.Flags().StringVarP(&opts.output, "output", "o", "", "write the snapshot transactionally to a file instead of stdout")
	command.Flags().StringVar(&opts.preset, "preset", "", "built-in preset: ai, compact, docs, monorepo, or none")
	command.Flags().VarP(&opts.depth, "depth", "d", "maximum depth (0 shows only the root; unlimited removes the limit)")
	command.Flags().BoolVar(&opts.directoriesOnly, "dirs-only", false, "include directories only")
	command.Flags().BoolVar(&opts.includeHidden, "hidden", false, "include hidden entries that survive other filters")
	command.Flags().StringArrayVar(&opts.ignorePatterns, "ignore", nil, "exclude a pattern (repeatable)")
	command.Flags().BoolVar(&opts.noDefaultIgnore, "no-default-ignore", false, "disable built-in directory exclusions")
	command.Flags().BoolVar(&opts.noGitIgnore, "no-gitignore", false, "do not apply .gitignore files")
	command.Flags().StringVar(&opts.color, "color", "", "accepted and ignored: snapshot identity is presentation-independent")
	command.Flags().StringVar(&opts.icons, "icons", "", "accepted and ignored: snapshot identity is presentation-independent")
	command.Flags().StringVar(&opts.theme, "theme", "", "accepted and ignored: snapshot identity is presentation-independent")
	registerSnapshotCompletions(command)
	command.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})
	return command
}

func registerSnapshotCompletions(command *cobra.Command) {
	fixed := func(values []string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return cobra.FixedCompletions(values, cobra.ShellCompDirectiveNoFileComp)
	}
	_ = command.RegisterFlagCompletionFunc("preset", fixed(append(append([]string{}, configuration.PresetNames()...), configuration.PresetNone)))
	_ = command.RegisterFlagCompletionFunc("depth", fixed([]string{"unlimited"}))
	_ = command.RegisterFlagCompletionFunc("color", fixed(presentation.ColorModes()))
	_ = command.RegisterFlagCompletionFunc("icons", fixed(presentation.IconModes()))
	_ = command.RegisterFlagCompletionFunc("theme", completeTheme)
	_ = command.RegisterFlagCompletionFunc("root", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveFilterDirs
	})
	_ = command.RegisterFlagCompletionFunc("output", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveDefault
	})
}

func classifySnapshotError(err error) error {
	if configuration.IsInvalid(err) {
		return &usageError{err: err}
	}
	if artifact.IsInternal(err) || snapshot.IsInternal(err) {
		return fmt.Errorf("internal error: %w", err)
	}
	return err
}
