package cli

import (
	"fmt"
	"io"
	"os"

	"github.com/dirloom/dirloom/internal/app"
	"github.com/dirloom/dirloom/internal/snapshot"
	"github.com/spf13/cobra"
)

type verifyOptions struct {
	root   string
	format string
}

func newVerifyCommand(stdout io.Writer) *cobra.Command {
	var opts verifyOptions
	command := &cobra.Command{
		Use:   "verify <snapshot> [directory]",
		Short: "Check whether a live tree still matches a structural snapshot",
		Long: "Compare the structure currently observed under the selected root with a\n" +
			"validated Snapshot Schema v1 file. The comparison is Fingerprint v1 equality.\n" +
			"Capture Semantics stored in the snapshot select the live view. Current\n" +
			"project and user configuration are not read. File contents are not hashed.\n" +
			"A structural mismatch is a normal result, not an error.",
		Example: `  dirloom verify architecture.dlm.json
  dirloom verify architecture.dlm.json .
  dirloom verify architecture.dlm.json ./src
  dirloom verify architecture.dlm.json --root ./src
  dirloom verify architecture.dlm.json --format json`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) < 1 {
				return &usageError{err: fmt.Errorf("verify requires a snapshot path")}
			}
			if args[0] == "" {
				return &usageError{err: fmt.Errorf("verify requires a non-empty snapshot path")}
			}
			if len(args) > 2 {
				return &usageError{err: fmt.Errorf("expected at most one directory argument, received %d", len(args)-1)}
			}
			return nil
		},
		ValidArgsFunction: completeVerifyArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rejectVerifyInheritedFlags(cmd); err != nil {
				return err
			}
			root, err := structuralRoot(cmd, args[1:], opts.root)
			if err != nil {
				return err
			}
			if opts.format != "text" && opts.format != "json" {
				return &usageError{err: fmt.Errorf("unsupported verify format %q (expected text or json)", opts.format)}
			}
			// #nosec G304 -- the snapshot path is the explicit verify reference selected by the caller.
			file, err := os.Open(args[0])
			if err != nil {
				return renderVerifyFailure(stdout, opts.format, app.FailureSnapshotRead, &snapshot.Error{
					Code:    snapshot.CodeReadFailure,
					Message: "snapshot read failure",
					Cause:   err,
				}, app.VerifyResult{})
			}
			defer func() { _ = file.Close() }()
			result, verifyErr := app.Verify(cmd.Context(), app.VerifyRequest{
				Snapshot:     file,
				SnapshotPath: args[0],
				Root:         root,
			})
			if verifyErr != nil {
				return renderVerifyFailure(stdout, opts.format, app.ClassifyVerifyFailure(verifyErr), verifyErr, result)
			}
			return renderVerifySuccess(stdout, opts.format, result)
		},
	}
	command.Flags().StringVar(&opts.root, "root", "", "directory to verify (default: positional argument or current directory)")
	command.Flags().StringVar(&opts.format, "format", "text", "output format: text or json")
	registerVerifyCompletions(command)
	command.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})
	return command
}

func rejectVerifyInheritedFlags(command *cobra.Command) error {
	names := []string{"config", "no-user-config", "no-config"}
	for _, name := range names {
		changed := command.Flags().Changed(name)
		if command.InheritedFlags() != nil && command.InheritedFlags().Changed(name) {
			changed = true
		}
		if changed {
			return &usageError{err: fmt.Errorf("--%s is not supported by verify; snapshot capture semantics are authoritative", name)}
		}
	}
	return nil
}

func completeVerifyArgs(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	switch len(args) {
	case 0:
		return nil, cobra.ShellCompDirectiveDefault
	case 1:
		return nil, cobra.ShellCompDirectiveFilterDirs
	default:
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
}

func registerVerifyCompletions(command *cobra.Command) {
	_ = command.RegisterFlagCompletionFunc("format", cobra.FixedCompletions([]string{"text", "json"}, cobra.ShellCompDirectiveNoFileComp))
	_ = command.RegisterFlagCompletionFunc("root", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveFilterDirs
	})
}

func renderVerifySuccess(stdout io.Writer, format string, result app.VerifyResult) error {
	if format == "json" {
		if err := writeVerifyJSON(stdout, verifySuccessDocument(result)); err != nil {
			return verifyWriteError(err)
		}
	} else if err := writeVerifyText(stdout, result); err != nil {
		return verifyWriteError(err)
	}
	if result.Status == app.VerifyMismatch {
		return &exitError{Code: 1, Silent: true}
	}
	return nil
}

func renderVerifyFailure(stdout io.Writer, format string, kind app.VerifyFailureKind, err error, result app.VerifyResult) error {
	if kind == "" {
		kind = app.FailureInternal
	}
	code := verifyExitCode(kind)
	if format == "json" {
		if writeErr := writeVerifyJSON(stdout, verifyFailureDocument(kind, err, result.ExpectedFingerprint)); writeErr != nil {
			return verifyWriteError(writeErr)
		}
		return &exitError{Code: code, Silent: true}
	}
	return &exitError{Code: code, Err: verifyHumanError(kind, err)}
}

func verifyExitCode(kind app.VerifyFailureKind) int {
	switch kind {
	case app.FailureInvalidSnapshot:
		return 3
	case app.FailureUnsupportedSnapshot:
		return 4
	case app.FailureSnapshotRead, app.FailureObservation:
		return 5
	default:
		return 6
	}
}

func verifyHumanError(kind app.VerifyFailureKind, err error) error {
	switch kind {
	case app.FailureInvalidSnapshot:
		return fmt.Errorf("invalid snapshot: %w", err)
	case app.FailureUnsupportedSnapshot:
		return fmt.Errorf("unsupported snapshot: %w", err)
	case app.FailureInternal:
		return fmt.Errorf("internal error: %w", err)
	default:
		return err
	}
}

func verifyWriteError(err error) error {
	return &exitError{Code: 5, Err: fmt.Errorf("write verify result: %w", err)}
}
