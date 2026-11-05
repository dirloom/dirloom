package cli

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/dirloom/dirloom/internal/app"
	"github.com/dirloom/dirloom/internal/comparison"
	configuration "github.com/dirloom/dirloom/internal/config"
	"github.com/dirloom/dirloom/internal/watch"
	"github.com/spf13/cobra"
)

type watchOptions struct {
	root            string
	format          string
	preset          string
	depth           optionalDepth
	directoriesOnly bool
	includeHidden   bool
	ignorePatterns  []string
	noDefaultIgnore bool
	noGitIgnore     bool
	noMoves         bool
	debounce        time.Duration
	poll            time.Duration
}

// watchRefusedFlags are the flags other commands take that watch refuses:
// its stream has no presentation options and no other destination.
var watchRefusedFlags = joinRefusedFlags(presentationFlags(), destinationFlags("it streams to stdout only"))

// watchSourcePrefixes are the source operands of diff and history: a watch
// observes one live directory, named as a plain path.
var watchSourcePrefixes = []string{"git:", "snapshot:", "live:"}

func newWatchCommand(stdout, stderr io.Writer, loader *configuration.Loader, sources *sourceOptions) *cobra.Command {
	var opts watchOptions
	command := &cobra.Command{
		Use:   "watch [directory]",
		Short: "EXPERIMENTAL: report structural changes of a directory as they happen",
		Long: "EXPERIMENTAL: Structural Watch v1 and Watch Event Schema v1 may change in any\n" +
			"v0.x release, with a new schema version and a CHANGELOG entry.\n\n" +
			"Watch a directory and report its structural transitions: each time the\n" +
			"structure Dirloom observes changes, the changes between the last reported\n" +
			"state and the new one, as one numbered generation, the same changes as\n" +
			"\"dirloom diff\" between the two states. File contents, timestamps and the\n" +
			"filesystem operations themselves are not reported: an edit in place reports\n" +
			"nothing, and several operations may be reported as their net effect.\n\n" +
			"Every observation scans the whole tree with the capture rules of\n" +
			"\"dirloom fingerprint\" (filters, preset, configuration). Native filesystem\n" +
			"notifications only decide when to observe, and a periodic reconciliation\n" +
			"catches what they miss. --poll observes at a fixed interval instead.\n\n" +
			"Native mode refuses recognized network and virtual filesystems (NFS, SMB,\n" +
			"AFP, WebDAV, 9p, ...); use --poll there. System limits: on Linux the inotify\n" +
			"limits (fs.inotify.max_user_watches); on macOS one descriptor per watched\n" +
			"directory (kern.maxfilesperproc); on Windows the root's parent directories\n" +
			"cannot be renamed while a native watch runs.\n\n" +
			"With --format ndjson, stdout carries one JSON event per line. Commit a\n" +
			"generation when its state.changed line arrives, discard a generation cut\n" +
			"short by the end of the stream, and treat a stream without watch.stopped as\n" +
			"incomplete. Ctrl+C or SIGTERM ends the watch with exit status 0; a usage\n" +
			"error exits 2, a fatal error 5 and an internal error 6.",
		Example: `  dirloom watch
  dirloom watch ./src --depth 3
  dirloom watch --format ndjson
  dirloom watch --no-moves
  dirloom watch --debounce 500ms
  dirloom watch --poll 2s
  dirloom watch --preset monorepo --ignore dist --no-gitignore`,
		Args: func(_ *cobra.Command, args []string) error {
			if len(args) > 1 {
				return &usageError{err: fmt.Errorf("expected at most one directory argument, received %d", len(args))}
			}
			return nil
		},
		ValidArgsFunction: completeInspectRoot,
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := validateWatchFlags(cmd, args, opts); err != nil {
				return err
			}
			root, err := structuralRoot(cmd, args, opts.root)
			if err != nil {
				return err
			}
			var sink watch.Sink = watch.NewTextSink(stdout, stderr)
			if opts.format == "ndjson" {
				sink = watch.NewNDJSONSink(stdout)
			}
			// SIGTERM ends a watch like Ctrl+C, and only a watch; on Windows,
			// Go delivers a console close, logoff or shutdown as SIGTERM.
			ctx, stop := signal.NotifyContext(cmd.Context(), syscall.SIGTERM)
			defer stop()
			if info, err := os.Stat(root); err != nil || !info.IsDir() {
				// The configuration cannot be resolved without its root: the
				// watch itself reports the startup failure (watch_root_unavailable).
				return watchStatus(app.Watch(ctx, app.WatchRequest{
					Inspect: app.InspectRequest{Root: root}, Debounce: opts.debounce, Poll: opts.poll,
				}, sink))
			}
			resolved, _, err := resolveStructuralOptions(cmd, loader, root, *sources, structuralFlagState{
				preset:          opts.preset,
				depth:           opts.depth,
				directoriesOnly: opts.directoriesOnly,
				includeHidden:   opts.includeHidden,
				ignorePatterns:  opts.ignorePatterns,
				noDefaultIgnore: opts.noDefaultIgnore,
				noGitIgnore:     opts.noGitIgnore,
			})
			if err != nil {
				// A watch has no exit status 1: a configuration that cannot be
				// read stops it before any output, as an invalid one does.
				var usage *usageError
				if !errors.As(err, &usage) {
					err = &usageError{err: err}
				}
				return err
			}
			request := app.WatchRequest{
				Inspect: app.InspectRequest{
					Root:              resolved.Root,
					MaxDepth:          resolved.Effective.MaxDepth,
					DirectoriesOnly:   resolved.Effective.DirectoriesOnly,
					IncludeHidden:     resolved.Effective.IncludeHidden,
					IgnorePatterns:    resolved.Effective.IgnorePatterns,
					UseDefaultIgnores: resolved.Effective.UseDefaultIgnores,
					UseGitIgnore:      resolved.Effective.UseGitIgnore,
				},
				Debounce: opts.debounce,
				Poll:     opts.poll,
			}
			if opts.noMoves {
				request.MoveDetection = comparison.MoveDetectionNone
			}
			return watchStatus(app.Watch(ctx, request, sink))
		},
	}
	flags := command.Flags()
	flags.StringVar(&opts.root, "root", "", "directory to watch (default: positional argument or current directory)")
	flags.StringVar(&opts.format, "format", "text", "output format: text or ndjson (one JSON event per line)")
	flags.StringVar(&opts.preset, "preset", "", "built-in preset: ai, compact, docs, monorepo, or none")
	flags.VarP(&opts.depth, "depth", "d", depthUsage)
	flags.BoolVar(&opts.directoriesOnly, "dirs-only", false, dirsOnlyUsage)
	flags.BoolVar(&opts.includeHidden, "hidden", false, "include hidden entries that survive other filters")
	flags.StringArrayVar(&opts.ignorePatterns, "ignore", nil, "exclude a pattern (repeatable)")
	flags.BoolVar(&opts.noDefaultIgnore, "no-default-ignore", false, "disable built-in directory exclusions")
	flags.BoolVar(&opts.noGitIgnore, "no-gitignore", false, "do not apply .gitignore files")
	flags.BoolVar(&opts.noMoves, "no-moves", false, noMovesUsage)
	flags.DurationVar(&opts.debounce, "debounce", 0, fmt.Sprintf("quiet time after a notification before observing, %s to %s (default %s; native mode)",
		watch.MinDebounce, watch.MaxDebounce, watch.DefaultDebounce))
	flags.DurationVar(&opts.poll, "poll", 0, fmt.Sprintf("observe every interval, %s to %s, instead of using notifications", watch.MinPoll, watch.MaxPoll))
	// Refused with a reason rather than left unknown; hidden from help and
	// completion.
	registerRefusedFlags(command, watchRefusedFlags)
	registerWatchCompletions(command)
	command.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})
	return command
}

// validateWatchFlags rejects what watch refuses before any configuration
// is read. The ranges of --debounce and --poll, and their combination, are
// app.Watch's to check.
func validateWatchFlags(cmd *cobra.Command, args []string, opts watchOptions) error {
	if err := rejectRefusedFlags(cmd, "watch", watchRefusedFlags); err != nil {
		return err
	}
	for _, arg := range args {
		for _, prefix := range watchSourcePrefixes {
			if strings.HasPrefix(arg, prefix) {
				return &usageError{err: fmt.Errorf("watch takes a directory, not a %s source operand: %q", strings.TrimSuffix(prefix, ":"), arg)}
			}
		}
	}
	formats := []string{"text", "ndjson"}
	if opts.format == "json" {
		invalid := invalidEnumeratedFlag("format", opts.format, formats, "watch")
		invalid.hint = "A watch streams events: use --format ndjson for one JSON object per line."
		return invalid
	}
	if err := requireEnumeratedFlag("format", opts.format, formats, "watch"); err != nil {
		return err
	}
	for _, flag := range []struct {
		name     string
		value    time.Duration
		min, max time.Duration
	}{
		{"debounce", opts.debounce, watch.MinDebounce, watch.MaxDebounce},
		{"poll", opts.poll, watch.MinPoll, watch.MaxPoll},
	} {
		if cmd.Flags().Changed(flag.name) && flag.value <= 0 {
			return &usageError{err: fmt.Errorf("--%s must be between %s and %s, received %s", flag.name, flag.min, flag.max, flag.value)}
		}
	}
	return nil
}

// watchStatus maps the end of a watch to its exit status: 0 interrupted, 2
// usage, 5 fatal or output failure, 6 internal. A fatal error is already in
// the stream (and on stderr in text mode), so it is not repeated.
func watchStatus(err error) error {
	var failure *watch.Failure
	var output *watch.OutputError
	switch {
	case err == nil:
		return nil
	case app.WatchUsage(err):
		return &usageError{err: err}
	case errors.As(err, &failure) && failure.Internal:
		return &exitError{Code: 6, Err: err, Silent: true}
	case errors.As(err, &failure):
		return &exitError{Code: 5, Err: err, Silent: true}
	case errors.As(err, &output), errors.Is(err, watch.ErrOutputBlocked):
		return &exitError{Code: 5, Err: err}
	default:
		return &exitError{Code: 6, Err: fmt.Errorf("internal error: %w", err)}
	}
}

func registerWatchCompletions(command *cobra.Command) {
	fixed := func(values []string) func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return cobra.FixedCompletions(values, cobra.ShellCompDirectiveNoFileComp)
	}
	_ = command.RegisterFlagCompletionFunc("format", fixed([]string{"text", "ndjson"}))
	_ = command.RegisterFlagCompletionFunc("preset", fixed(append(append([]string{}, configuration.PresetNames()...), configuration.PresetNone)))
	_ = command.RegisterFlagCompletionFunc("depth", fixed([]string{"unlimited"}))
	_ = command.RegisterFlagCompletionFunc("debounce", fixed(nil))
	_ = command.RegisterFlagCompletionFunc("poll", fixed(nil))
	_ = command.RegisterFlagCompletionFunc("root", func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
		return nil, cobra.ShellCompDirectiveFilterDirs
	})
}
