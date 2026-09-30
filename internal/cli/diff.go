package cli

import (
	"fmt"
	"io"
	"strings"

	"github.com/dirloom/dirloom/internal/app"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/spf13/cobra"
)

type diffOptions struct {
	format string
}

func newDiffCommand(stdout io.Writer) *cobra.Command {
	var opts diffOptions
	command := &cobra.Command{
		Use:   "diff <source-a> <source-b>",
		Short: "Compare two structural sources and list what changed",
		Long: "Compare two structural sources through Identity Projection v1 and list the\n" +
			"added, removed, and changed canonical paths. A source is snapshot:<path> or\n" +
			"live:<directory>. A live side is observed with the Capture Semantics stored\n" +
			"in the opposite snapshot; current project and user configuration are not read.\n" +
			"File contents are not hashed. A rename or move is one REMOVED plus one ADDED.\n" +
			"Structural differences are a normal result, not an error.",
		Example: `  dirloom diff snapshot:architecture.dlm.json live:.
  dirloom diff live:. snapshot:architecture.dlm.json
  dirloom diff snapshot:before.dlm.json snapshot:after.dlm.json
  dirloom diff snapshot:architecture.dlm.json live:./src --format json`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 2 {
				return &usageError{err: fmt.Errorf("diff requires exactly two sources (snapshot:<path> or live:<directory>), received %d", len(args))}
			}
			return nil
		},
		ValidArgsFunction: completeDiffArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rejectDiffInheritedFlags(cmd); err != nil {
				return err
			}
			if opts.format != "text" && opts.format != "json" {
				return &usageError{err: fmt.Errorf("unsupported diff format %q (expected text or json)", opts.format)}
			}
			specA, err := app.ParseDiffSource(args[0])
			if err != nil {
				return &usageError{err: err}
			}
			specB, err := app.ParseDiffSource(args[1])
			if err != nil {
				return &usageError{err: err}
			}
			result, diffErr := app.Diff(cmd.Context(), app.DiffRequest{A: specA, B: specB})
			if diffErr != nil {
				if app.DiffUsage(diffErr) {
					return &usageError{err: diffErr}
				}
				return renderDiffFailure(stdout, opts.format, app.ClassifyDiffFailure(diffErr), diffErr)
			}
			return renderDiffSuccess(stdout, opts.format, result)
		},
	}
	command.Flags().StringVar(&opts.format, "format", "text", "output format: text or json")
	registerDiffCompletions(command)
	command.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})
	return command
}

func rejectDiffInheritedFlags(command *cobra.Command) error {
	names := []string{"config", "no-user-config", "no-config"}
	for _, name := range names {
		changed := command.Flags().Changed(name)
		if command.InheritedFlags() != nil && command.InheritedFlags().Changed(name) {
			changed = true
		}
		if changed {
			return &usageError{err: fmt.Errorf("--%s is not supported by diff; snapshot capture semantics are authoritative", name)}
		}
	}
	return nil
}

func completeDiffArgs(_ *cobra.Command, args []string, toComplete string) ([]string, cobra.ShellCompDirective) {
	if len(args) >= 2 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	if strings.HasPrefix(toComplete, "live:") {
		return nil, cobra.ShellCompDirectiveFilterDirs
	}
	if strings.HasPrefix(toComplete, "snapshot:") {
		return nil, cobra.ShellCompDirectiveDefault
	}
	return []string{"live:", "snapshot:"}, cobra.ShellCompDirectiveNoSpace
}

func registerDiffCompletions(command *cobra.Command) {
	_ = command.RegisterFlagCompletionFunc("format", cobra.FixedCompletions([]string{"text", "json"}, cobra.ShellCompDirectiveNoFileComp))
}

func renderDiffSuccess(stdout io.Writer, format string, result comparison.StructuralDiff) error {
	if format == "json" {
		if err := writeDiffJSON(stdout, diffSuccessDocument(result)); err != nil {
			return diffWriteError(err)
		}
	} else if err := writeDiffText(stdout, result); err != nil {
		return diffWriteError(err)
	}
	if app.DiffStatusOf(result) == app.DiffDifferences {
		return &exitError{Code: 1, Silent: true}
	}
	return nil
}

// renderDiffFailure reuses the verify exit-code and human-diagnostic helpers:
// diff shares the verify failure taxonomy (exits 3, 4, 5, 6).
func renderDiffFailure(stdout io.Writer, format string, kind app.VerifyFailureKind, err error) error {
	if kind == "" {
		kind = app.FailureInternal
	}
	code := verifyExitCode(kind)
	if format == "json" {
		if writeErr := writeDiffJSON(stdout, diffFailureDocument(kind, err)); writeErr != nil {
			return diffWriteError(writeErr)
		}
		return &exitError{Code: code, Silent: true}
	}
	return &exitError{Code: code, Err: verifyHumanError(kind, err)}
}

func diffWriteError(err error) error {
	return &exitError{Code: 5, Err: fmt.Errorf("write diff result: %w", err)}
}
