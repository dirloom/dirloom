// Package cli implements Dirloom's non-interactive command-line interface.
package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/dirloom/dirloom/internal/app"
	"github.com/dirloom/dirloom/internal/clipboard"
	configuration "github.com/dirloom/dirloom/internal/config"
	"github.com/dirloom/dirloom/internal/diagram"
	"github.com/dirloom/dirloom/internal/output"
	"github.com/dirloom/dirloom/internal/outputformat"
	"github.com/dirloom/dirloom/internal/presentation"
	"github.com/dirloom/dirloom/internal/render"
	"github.com/spf13/cobra"
)

type usageError struct {
	err error
}

func (e *usageError) Error() string { return e.err.Error() }
func (e *usageError) Unwrap() error { return e.err }

type commandDependencies struct {
	loader    *configuration.Loader
	evaluator *presentation.Evaluator
	clipboard clipboard.Writer
}

// Execute runs the CLI with explicit streams and returns a stable process exit
// code: 0 success, 1 runtime error, 2 invalid arguments.
func Execute(ctx context.Context, args []string, stdout, stderr io.Writer, version string) int {
	return execute(ctx, args, stdout, stderr, version, commandDependencies{
		loader:    configuration.NewLoader(),
		evaluator: presentation.NewEvaluator(),
		clipboard: clipboard.New(),
	})
}

func executeWithLoader(ctx context.Context, args []string, stdout, stderr io.Writer, version string, loader *configuration.Loader) int {
	return execute(ctx, args, stdout, stderr, version, commandDependencies{
		loader:    loader,
		evaluator: presentation.NewEvaluator(),
		clipboard: &clipboard.Buffer{},
	})
}

func executeWithDependencies(ctx context.Context, args []string, stdout, stderr io.Writer, version string, loader *configuration.Loader, evaluator *presentation.Evaluator) int {
	return execute(ctx, args, stdout, stderr, version, commandDependencies{
		loader:    loader,
		evaluator: evaluator,
		clipboard: &clipboard.Buffer{},
	})
}

func execute(ctx context.Context, args []string, stdout, stderr io.Writer, version string, deps commandDependencies) int {
	command := newRootCommandWithRuntime(stdout, stderr, version, deps)
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
	return newRootCommand(stdout, stderr, version, configuration.NewLoader())
}

func newRootCommand(stdout, stderr io.Writer, version string, loader *configuration.Loader) *cobra.Command {
	return newRootCommandWithRuntime(stdout, stderr, version, commandDependencies{
		loader:    loader,
		evaluator: presentation.NewEvaluator(),
		clipboard: &clipboard.Buffer{},
	})
}

func newRootCommandWithRuntime(stdout, stderr io.Writer, version string, deps commandDependencies) *cobra.Command {
	var opts options
	var sources sourceOptions
	command := &cobra.Command{
		Use:   "dirloom [directory]",
		Short: "Create clean, deterministic project trees",
		Long: "Dirloom turns a directory into a clean, deterministic and shareable\n" +
			"tree for terminals, documentation, CI pipelines, humans and tools.\n\n" +
			"Arguments:\n  directory   Directory to inspect (default: current directory)",
		Example: `  dirloom
  dirloom ./src
  dirloom --depth 3
  dirloom --preset docs
  dirloom --dirs-only
  dirloom --style ascii
  dirloom --theme midnight --icons unicode
  dirloom --format markdown
  dirloom --format markdown --copy
  dirloom --format markdown-tree
  dirloom --format mermaid --diagram-direction left-right
  dirloom --format graphviz --output structure.dot
  dirloom --format d2 --output structure.d2
  dirloom --ignore node_modules --ignore dist
  dirloom --output structure.md --format markdown
  dirloom completion bash`,
		Version:       version,
		SilenceErrors: true,
		SilenceUsage:  true,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return &usageError{err: fmt.Errorf("expected at most one directory argument, received %d", len(args))}
			}
			return nil
		},
		ValidArgsFunction: completeInspectRoot,
		RunE: func(cmd *cobra.Command, args []string) error {
			if opts.copy && cmd.Flags().Changed("output") {
				return &usageError{err: fmt.Errorf("--copy and --output are mutually exclusive")}
			}
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			resolved, overrides, err := resolveOptions(cmd, deps.loader, root, sources, &opts)
			if err != nil {
				return err
			}
			if err := validateFormatOptions(resolved, overrides); err != nil {
				return err
			}
			theme, compiled, err := loadEffectiveTheme(resolved)
			if err != nil {
				return classifyPresentationError(err)
			}
			resolved.SetThemeInfo(theme)
			capabilities, err := deps.evaluator.Evaluate(presentation.CapabilityRequest{
				Format: resolved.Effective.Format, ColorMode: resolved.Effective.Color, IconMode: resolved.Effective.Icons,
				ColorExplicitCLI: overrides.Color.Set, OutputPath: opts.output, Clipboard: opts.copy, Writer: stdout,
			})
			if err != nil {
				return classifyPresentationError(err)
			}
			closed := false
			defer func() {
				if !closed {
					_ = capabilities.Close()
				}
			}()

			model, err := app.Inspect(cmd.Context(), app.InspectRequest{
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
				return err
			}

			var decorator render.Decorator
			if capabilities.ColorEnabled || capabilities.IconMode != presentation.IconsNever {
				decorator = presentation.NewDecorator(compiled, capabilities.ColorEnabled, capabilities.IconMode, capabilities.Profile)
			}
			nodeCount := diagram.CountNodes(model)
			if !opts.copy && outputformat.IsDiagram(resolved.Effective.Format) && resolved.Effective.DiagramMaxNodes == nil &&
				nodeCount >= diagram.LargeGraphWarningThreshold {
				_, _ = fmt.Fprintf(stderr, "Warning: diagram contains %d nodes; consider --depth, --dirs-only, --ignore, or an explicit --diagram-max-nodes limit\n", nodeCount)
			}
			renderer, err := render.NewConfigured(render.Options{
				Format: resolved.Effective.Format, Style: resolved.Effective.Style, Decorator: decorator,
				Diagram: diagram.Options{
					View:      diagram.View(resolved.Effective.DiagramView),
					Direction: diagram.Direction(resolved.Effective.DiagramDirection),
					MaxNodes:  resolved.Effective.DiagramMaxNodes,
				},
			})
			if err != nil {
				return err
			}
			var rendered bytes.Buffer
			if err := renderer.Render(&rendered, model); err != nil {
				return fmt.Errorf("render tree: %w", err)
			}
			if err := output.ValidateText(rendered.Bytes()); err != nil {
				return err
			}
			if err := writeRendered(opts, deps.clipboard, stdout, rendered.Bytes()); err != nil {
				return err
			}
			if err := capabilities.Close(); err != nil {
				return err
			}
			closed = true
			return nil
		},
	}

	command.SetOut(stdout)
	command.SetErr(stderr)
	command.CompletionOptions.DisableDefaultCmd = true
	command.SetVersionTemplate("dirloom {{.Version}}\n")
	command.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})
	command.PersistentFlags().StringVar(&sources.path, "config", "", "use a project configuration file instead of automatic discovery")
	command.PersistentFlags().BoolVar(&sources.noUser, "no-user-config", false, "do not load the user configuration file")
	command.PersistentFlags().BoolVar(&sources.noConfig, "no-config", false, "do not load user or project configuration files")
	bindInspectFlags(command, &opts)
	command.Flags().StringVarP(&opts.output, "output", "o", "", "write transactionally to a file instead of stdout")
	command.Flags().BoolVar(&opts.copy, "copy", false, "copy the rendered tree to the clipboard instead of stdout")
	registerInspectCompletions(command)
	command.AddCommand(newConfigCommand(stdout, deps.loader, &sources))
	command.AddCommand(newPresetCommand(stdout, &sources))
	command.AddCommand(newThemeCommand(stdout, &sources))
	command.AddCommand(newCompletionCommand(stdout))
	return command
}

func writeRendered(opts options, clip clipboard.Writer, stdout io.Writer, data []byte) error {
	switch {
	case opts.copy:
		if err := clip.Write(data); err != nil {
			return fmt.Errorf("copy to clipboard: %w", err)
		}
		return nil
	case opts.output != "":
		return output.WriteFile(opts.output, data)
	default:
		if err := writeAll(stdout, data); err != nil {
			return fmt.Errorf("write output: %w", err)
		}
		return nil
	}
}

func bindInspectFlags(command *cobra.Command, opts *options) {
	command.Flags().StringVar(&opts.preset, "preset", "", "built-in preset: ai, compact, docs, monorepo, or none")
	command.Flags().VarP(&opts.depth, "depth", "d", "maximum depth (0 shows only the root; unlimited removes the limit)")
	command.Flags().BoolVar(&opts.directoriesOnly, "dirs-only", false, "show directories only")
	command.Flags().BoolVar(&opts.includeHidden, "hidden", false, "include hidden entries that survive other filters")
	command.Flags().StringArrayVar(&opts.ignorePatterns, "ignore", nil, "exclude a pattern (repeatable)")
	command.Flags().BoolVar(&opts.noDefaultIgnore, "no-default-ignore", false, "disable built-in directory exclusions")
	command.Flags().BoolVar(&opts.noGitIgnore, "no-gitignore", false, "do not apply .gitignore files")
	command.Flags().StringVar(&opts.format, "format", "", "output format: text, markdown, markdown-tree, json, mermaid, graphviz (dot), or d2")
	command.Flags().StringVar(&opts.style, "style", "", "tree style: unicode or ascii")
	command.Flags().StringVar(&opts.color, "color", "", "terminal colors: never, always, or auto")
	command.Flags().StringVar(&opts.icons, "icons", "", "terminal icons: never, unicode, nerd, or auto")
	command.Flags().StringVar(&opts.theme, "theme", "", "terminal theme: default, midnight, daylight, vivid, or a YAML path")
	command.Flags().StringVar(&opts.diagramView, "diagram-view", "", "diagram view: structure")
	command.Flags().StringVar(&opts.diagramDirection, "diagram-direction", "", "diagram direction: top-down or left-right")
	command.Flags().Var(&opts.diagramMaxNodes, "diagram-max-nodes", "maximum diagram nodes (positive integer or unlimited)")
}

func resolveOptions(command *cobra.Command, loader *configuration.Loader, root string, sources sourceOptions, opts *options) (configuration.Resolution, configuration.Overrides, error) {
	if command.Flags().Changed("config") && sources.path == "" {
		return configuration.Resolution{}, configuration.Overrides{}, &usageError{err: fmt.Errorf("--config requires a non-empty path")}
	}
	overrides, err := explicitOverrides(command, opts)
	if err != nil {
		return configuration.Resolution{}, configuration.Overrides{}, err
	}
	resolved, err := loader.Resolve(configuration.ResolveOptions{
		Root:                root,
		ExplicitProjectPath: sources.path,
		DisableUser:         sources.noUser,
		DisableAll:          sources.noConfig,
		Overrides:           overrides,
	})
	if err != nil {
		if configuration.IsInvalid(err) {
			return configuration.Resolution{}, configuration.Overrides{}, &usageError{err: err}
		}
		return configuration.Resolution{}, configuration.Overrides{}, err
	}
	return resolved, overrides, nil
}

func explicitOverrides(command *cobra.Command, opts *options) (configuration.Overrides, error) {
	result := configuration.Overrides{}
	if command.Flags().Changed("preset") {
		if opts.preset == "" {
			return configuration.Overrides{}, &usageError{err: fmt.Errorf("--preset requires a non-empty value")}
		}
		if opts.preset == configuration.PresetNone {
			result.Preset = configuration.PresetSelection{Set: true, Disabled: true}
		} else {
			result.Preset = configuration.PresetSelection{Set: true, Name: opts.preset}
		}
	}
	if command.Flags().Changed("depth") {
		result.Depth = configuration.DepthOverride{Set: true, Unlimited: opts.depth.unlimited, Value: opts.depth.value}
	}
	if command.Flags().Changed("dirs-only") {
		result.DirectoriesOnly = configuration.Optional[bool]{Set: true, Value: opts.directoriesOnly}
	}
	if command.Flags().Changed("hidden") {
		result.IncludeHidden = configuration.Optional[bool]{Set: true, Value: opts.includeHidden}
	}
	if command.Flags().Changed("format") {
		canonical, ok := outputformat.Canonical(opts.format)
		if !ok {
			return configuration.Overrides{}, &usageError{err: outputformat.Validate(opts.format)}
		}
		result.Format = configuration.Optional[string]{Set: true, Value: canonical}
	}
	if command.Flags().Changed("style") {
		result.Style = configuration.Optional[string]{Set: true, Value: opts.style}
	}
	if command.Flags().Changed("no-default-ignore") {
		result.UseDefaultIgnores = configuration.Optional[bool]{Set: true, Value: !opts.noDefaultIgnore}
	}
	if command.Flags().Changed("no-gitignore") {
		result.UseGitIgnore = configuration.Optional[bool]{Set: true, Value: !opts.noGitIgnore}
	}
	if command.Flags().Changed("ignore") {
		result.IgnorePatterns = append([]string(nil), opts.ignorePatterns...)
	}
	if command.Flags().Changed("color") {
		if opts.color == "" {
			return configuration.Overrides{}, &usageError{err: fmt.Errorf("--color requires a non-empty value")}
		}
		result.Color = configuration.Optional[string]{Set: true, Value: opts.color}
	}
	if command.Flags().Changed("icons") {
		if opts.icons == "" {
			return configuration.Overrides{}, &usageError{err: fmt.Errorf("--icons requires a non-empty value")}
		}
		result.Icons = configuration.Optional[string]{Set: true, Value: opts.icons}
	}
	if command.Flags().Changed("theme") {
		if opts.theme == "" {
			return configuration.Overrides{}, &usageError{err: fmt.Errorf("--theme requires a non-empty value")}
		}
		result.Theme = configuration.ThemeSelection{Set: true, Value: opts.theme}
	}
	if command.Flags().Changed("diagram-view") {
		if opts.diagramView == "" {
			return configuration.Overrides{}, &usageError{err: fmt.Errorf("--diagram-view requires a non-empty value")}
		}
		result.DiagramView = configuration.Optional[string]{Set: true, Value: opts.diagramView}
	}
	if command.Flags().Changed("diagram-direction") {
		if opts.diagramDirection == "" {
			return configuration.Overrides{}, &usageError{err: fmt.Errorf("--diagram-direction requires a non-empty value")}
		}
		result.DiagramDirection = configuration.Optional[string]{Set: true, Value: opts.diagramDirection}
	}
	if command.Flags().Changed("diagram-max-nodes") {
		result.DiagramMaxNodes = configuration.LimitOverride{
			Set: true, Unlimited: opts.diagramMaxNodes.unlimited, Value: opts.diagramMaxNodes.value,
		}
	}
	return result, nil
}

func newConfigCommand(stdout io.Writer, loader *configuration.Loader, sources *sourceOptions) *cobra.Command {
	command := &cobra.Command{
		Use:   "config",
		Short: "Inspect Dirloom configuration",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 {
				return &usageError{err: fmt.Errorf("expected a config subcommand, received %d argument(s)", len(args))}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			return cmd.Help()
		},
	}
	var opts options
	var outputFormat string
	explain := &cobra.Command{
		Use:   "explain [directory]",
		Short: "Explain effective values and their sources",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return &usageError{err: fmt.Errorf("expected at most one directory argument, received %d", len(args))}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			root := "."
			if len(args) == 1 {
				root = args[0]
			}
			resolved, overrides, err := resolveOptions(cmd, loader, root, *sources, &opts)
			if err != nil {
				return err
			}
			if err := validateFormatOptions(resolved, overrides); err != nil {
				return err
			}
			theme, _, err := loadEffectiveTheme(resolved)
			if err != nil {
				return classifyPresentationError(err)
			}
			resolved.SetThemeInfo(theme)
			switch outputFormat {
			case "text":
				if err := resolved.WriteText(stdout); err != nil {
					return fmt.Errorf("write configuration explanation: %w", err)
				}
			case "json":
				if err := resolved.WriteJSON(stdout); err != nil {
					return fmt.Errorf("write configuration explanation: %w", err)
				}
			default:
				return &usageError{err: fmt.Errorf("unsupported explanation format %q (expected text or json)", outputFormat)}
			}
			return nil
		},
	}
	bindInspectFlags(explain, &opts)
	registerInspectCompletions(explain)
	explain.Flags().StringVar(&outputFormat, "as", "text", "explanation format: text or json")
	registerAsCompletions(explain)
	explain.ValidArgsFunction = completeInspectRoot
	command.AddCommand(explain)
	return command
}

func newPresetCommand(stdout io.Writer, sources *sourceOptions) *cobra.Command {
	names := strings.Join(configuration.PresetNames(), ", ")
	command := &cobra.Command{
		Use:   "preset",
		Short: "Inspect built-in presets",
		Long:  "Inspect Dirloom's built-in presets. Available presets: " + names + ".",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 0 {
				return &usageError{err: fmt.Errorf("expected a preset subcommand, received %d argument(s)", len(args))}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := rejectPresetSourceFlags(cmd, sources); err != nil {
				return err
			}
			return cmd.Help()
		},
	}
	var outputFormat string
	explain := &cobra.Command{
		Use:   "explain <preset>",
		Short: "Explain one built-in preset",
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) != 1 {
				return &usageError{err: fmt.Errorf("expected exactly one preset name, received %d", len(args))}
			}
			return nil
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := rejectPresetSourceFlags(cmd, sources); err != nil {
				return err
			}
			definition, ok := configuration.LookupPreset(args[0])
			if !ok {
				return &usageError{err: fmt.Errorf("unsupported preset %q (expected %s)", args[0], names)}
			}
			switch outputFormat {
			case "text":
				if err := definition.WriteText(stdout); err != nil {
					return fmt.Errorf("write preset explanation: %w", err)
				}
			case "json":
				if err := definition.WriteJSON(stdout); err != nil {
					return fmt.Errorf("write preset explanation: %w", err)
				}
			default:
				return &usageError{err: fmt.Errorf("unsupported explanation format %q (expected text or json)", outputFormat)}
			}
			return nil
		},
	}
	explain.Flags().StringVar(&outputFormat, "as", "text", "explanation format: text or json")
	registerAsCompletions(explain)
	explain.ValidArgsFunction = func(_ *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
		if len(args) > 0 {
			return nil, cobra.ShellCompDirectiveNoFileComp
		}
		return configuration.PresetNames(), cobra.ShellCompDirectiveNoFileComp
	}
	command.AddCommand(explain)
	return command
}

func rejectPresetSourceFlags(command *cobra.Command, sources *sourceOptions) error {
	return rejectSourceFlags(command, sources, "preset inspection")
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
