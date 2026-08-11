// Package cli implements Dirloom's non-interactive command-line interface.
package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"

	"github.com/dirloom/dirloom/internal/app"
	"github.com/dirloom/dirloom/internal/filter"
	"github.com/dirloom/dirloom/internal/output"
	"github.com/dirloom/dirloom/internal/render"
	"github.com/spf13/cobra"
)

type usageError struct {
	err error
}

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

// Execute runs the CLI with explicit streams and returns a stable process exit
// code: 0 success, 1 runtime error, 2 invalid arguments.
func Execute(ctx context.Context, args []string, stdout, stderr io.Writer, version string) int {
	command := NewRootCommand(stdout, stderr, version)
	command.SetArgs(args)
	if err := command.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintf(stderr, "Error: %s\n", err)
		var invalid *usageError
		if errors.As(err, &invalid) {
			return 2
		}
		return 1
	}
	return 0
}

// NewRootCommand constructs the root Cobra command without process globals.
func NewRootCommand(stdout, stderr io.Writer, version string) *cobra.Command {
	var opts options
	command := &cobra.Command{
		Use:   "dirloom [directory]",
		Short: "Create clean, deterministic project trees",
		Long: "Dirloom turns a directory into a clean, deterministic and shareable\n" +
			"tree for terminals, documentation, CI pipelines, humans and tools.\n\n" +
			"Arguments:\n  directory   Directory to inspect (default: current directory)",
		Example: `  dirloom
  dirloom ./src
  dirloom --depth 3
  dirloom --dirs-only
  dirloom --style ascii
  dirloom --format markdown
  dirloom --ignore node_modules --ignore dist
  dirloom --output structure.md --format markdown`,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return &usageError{err: fmt.Errorf("expected at most one directory argument, received %d", len(args))}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateOptions(cmd, &opts); err != nil {
				return &usageError{err: err}
			}
			root := "."
			if len(args) == 1 {
				root = args[0]
			}

			model, err := app.Inspect(cmd.Context(), app.InspectRequest{
				Root:              root,
				MaxDepth:          opts.depth.Pointer(),
				DirectoriesOnly:   opts.directoriesOnly,
				IncludeHidden:     opts.includeHidden,
				IgnorePatterns:    opts.ignorePatterns,
				UseDefaultIgnores: !opts.noDefaultIgnore,
				UseGitIgnore:      !opts.noGitIgnore,
				OutputPath:        opts.output,
			})
			if err != nil {
				return err
			}

			renderer, err := render.New(opts.format, opts.style)
			if err != nil {
				return err
			}
			var rendered bytes.Buffer
			if err := renderer.Render(&rendered, model); err != nil {
				return fmt.Errorf("render tree: %w", err)
			}

			if opts.output != "" {
				return output.WriteFile(opts.output, rendered.Bytes())
			}
			if err := writeAll(stdout, rendered.Bytes()); err != nil {
				return fmt.Errorf("write output: %w", err)
			}
			return nil
		},
	}

	command.SetOut(stdout)
	command.SetErr(stderr)
	command.SetVersionTemplate("dirloom {{.Version}}\n")
	command.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})
	command.Flags().VarP(&opts.depth, "depth", "d", "maximum depth (0 shows only the root)")
	command.Flags().BoolVar(&opts.directoriesOnly, "dirs-only", false, "show directories only")
	command.Flags().BoolVar(&opts.includeHidden, "hidden", false, "include hidden entries that survive other filters")
	command.Flags().StringArrayVar(&opts.ignorePatterns, "ignore", nil, "exclude a pattern (repeatable)")
	command.Flags().BoolVar(&opts.noDefaultIgnore, "no-default-ignore", false, "disable built-in directory exclusions")
	command.Flags().BoolVar(&opts.noGitIgnore, "no-gitignore", false, "do not apply .gitignore files")
	command.Flags().StringVar(&opts.format, "format", render.FormatText, "output format: text, markdown, or json")
	command.Flags().StringVar(&opts.style, "style", render.StyleUnicode, "tree style: unicode or ascii")
	command.Flags().StringVarP(&opts.output, "output", "o", "", "write transactionally to a file instead of stdout")

	return command
}

func validateOptions(command *cobra.Command, opts *options) error {
	switch opts.format {
	case render.FormatText, render.FormatMarkdown, render.FormatJSON:
	default:
		return fmt.Errorf("unsupported format %q (expected text, markdown, or json)", opts.format)
	}
	switch opts.style {
	case render.StyleUnicode, render.StyleASCII:
	default:
		return fmt.Errorf("unsupported style %q (expected unicode or ascii)", opts.style)
	}
	if opts.format == render.FormatJSON && command.Flags().Changed("style") {
		return fmt.Errorf("--style cannot be used with --format json")
	}
	if _, err := filter.NewIgnoreMatcher(opts.ignorePatterns); err != nil {
		return err
	}
	return nil
}

func writeAll(writer io.Writer, data []byte) error {
	for len(data) > 0 {
		written, err := writer.Write(data)
		if err != nil {
			return err
		}
		if written == 0 {
			return io.ErrShortWrite
		}
		data = data[written:]
	}
	return nil
}
