package cli

import (
	"fmt"
	"io"

	"github.com/dirloom/dirloom/internal/app"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/history"
	"github.com/spf13/cobra"
)

type historyOptions struct {
	format         string
	from           string
	limit          int
	structuralOnly bool
	noMoves        bool
}

func newHistoryCommand(stdout io.Writer) *cobra.Command {
	var opts historyOptions
	command := &cobra.Command{
		Use:   "history [revision]",
		Short: "List how the structure changed along a Git history",
		Long: "List how the structure of a Git repository changed along the first-parent\n" +
			"history of a revision (default HEAD): one line per commit, with the number of\n" +
			"added, removed, changed, and moved paths against its first parent. A root\n" +
			"commit is compared with the empty tree. Every tree is observed whole, without\n" +
			"Capture Semantics. Git objects only are read: never the index or the working\n" +
			"tree, never the network.\n\n" +
			"This is a structural history, not git log: a commit that changes only file\n" +
			"contents is listed as \"no structural change\", and --structural-only hides it.\n" +
			"The walk follows first parents only and stops after --limit commits (default\n" +
			"20), before --from (exclusive), at a root commit, or at a shallow boundary;\n" +
			"the header says which. Commit messages and identities are never shown.\n" +
			"Requires git 2.45 or newer.",
		Example: `  dirloom history
  dirloom history main --limit 50 --structural-only
  dirloom history HEAD --from origin/main --format json
  dirloom history v0.4.0-a6 --no-moves`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return &usageError{err: fmt.Errorf("history accepts at most one revision, received %d", len(args))}
			}
			return nil
		},
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			// Revisions are not completed in v1.
			return nil, cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rejectHistoryInheritedFlags(cmd); err != nil {
				return err
			}
			if opts.format != "text" && opts.format != "json" {
				return &usageError{err: fmt.Errorf("unsupported history format %q (expected text or json)", opts.format)}
			}
			revision := history.DefaultRevision
			if len(args) == 1 {
				if args[0] == "" {
					return &usageError{err: fmt.Errorf("history revision must not be empty")}
				}
				revision = args[0]
			}
			if cmd.Flags().Changed("from") && opts.from == "" {
				return &usageError{err: fmt.Errorf("--from must not be empty")}
			}
			moveDetection := comparison.MoveDetectionStructuralV1
			if opts.noMoves {
				moveDetection = comparison.MoveDetectionNone
			}
			result, err := app.History(cmd.Context(), app.HistoryRequest{
				Revision:      revision,
				From:          opts.from,
				Limit:         opts.limit,
				MoveDetection: moveDetection,
			})
			if err != nil {
				if app.HistoryUsage(err) {
					return &usageError{err: err}
				}
				return renderHistoryFailure(stdout, opts.format, app.ClassifyHistoryFailure(err), err)
			}
			return renderHistorySuccess(stdout, opts.format, result, opts.structuralOnly)
		},
	}
	command.Flags().StringVar(&opts.format, "format", "text", "output format: text or json")
	command.Flags().StringVar(&opts.from, "from", "", "stop before this revision (exclusive bound on the first-parent chain)")
	command.Flags().IntVar(&opts.limit, "limit", history.DefaultLimit, "number of commits to examine")
	command.Flags().BoolVar(&opts.structuralOnly, "structural-only", false, "show only commits that change the structure")
	command.Flags().BoolVar(&opts.noMoves, "no-moves", false, "count moves as REMOVED and ADDED (disable move detection)")
	_ = command.RegisterFlagCompletionFunc("format", cobra.FixedCompletions([]string{"text", "json"}, cobra.ShellCompDirectiveNoFileComp))
	_ = command.RegisterFlagCompletionFunc("from", cobra.NoFileCompletions)
	_ = command.RegisterFlagCompletionFunc("limit", cobra.NoFileCompletions)
	command.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})
	return command
}

func rejectHistoryInheritedFlags(command *cobra.Command) error {
	for _, name := range []string{"config", "no-user-config", "no-config"} {
		changed := command.Flags().Changed(name)
		if command.InheritedFlags() != nil && command.InheritedFlags().Changed(name) {
			changed = true
		}
		if changed {
			return &usageError{err: fmt.Errorf("--%s is not supported by history; committed trees are observed without configuration", name)}
		}
	}
	return nil
}

func renderHistorySuccess(stdout io.Writer, format string, result history.Result, structuralOnly bool) error {
	entries := result.Entries
	if structuralOnly {
		entries = result.StructuralEntries()
	}
	var err error
	if format == "json" {
		err = writeHistoryJSON(stdout, historySuccessDocument(result, entries))
	} else {
		err = writeHistoryText(stdout, result, entries, structuralOnly)
	}
	if err != nil {
		return &exitError{Code: 5, Err: fmt.Errorf("write history result: %w", err)}
	}
	return nil
}

// renderHistoryFailure uses the verify exit codes and human diagnostics:
// history failures are observation (5) or internal (6) failures.
func renderHistoryFailure(stdout io.Writer, format string, kind app.VerifyFailureKind, err error) error {
	if kind == "" {
		kind = app.FailureInternal
	}
	code := verifyExitCode(kind)
	if format == "json" {
		if writeErr := writeHistoryJSON(stdout, historyFailureDocument(kind, err)); writeErr != nil {
			return &exitError{Code: 5, Err: fmt.Errorf("write history result: %w", writeErr)}
		}
		return &exitError{Code: code, Silent: true}
	}
	return &exitError{Code: code, Err: verifyHumanError(kind, err)}
}
