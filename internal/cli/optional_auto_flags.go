package cli

import (
	"strings"

	"github.com/dirloom/dirloom/internal/presentation"
	"github.com/spf13/cobra"
)

// normalizeOptionalAutoFlags rewrites bare --icons/--color so they mean auto
// without pflag NoOptDefVal, which would break `--icons unicode` and value
// completions.
//
// Contract:
//   - Rewriting stops at `--`; later tokens are never treated as Dirloom flags.
//   - A following flag (`--help`, `--depth`, `--icons`, …) is not consumed as a
//     mode; the optional flag becomes auto instead.
//   - A following public enum value stays a separated value (`--icons unicode`).
//   - Any other non-flag token stays a flag value so historical invalid usage
//     such as `--icons banana` still diagnoses the attempted mode instead of
//     becoming `--icons=auto` plus a directory named banana.
//   - Cobra completion keeps an empty or partial toComplete token separate.
func normalizeOptionalAutoFlags(args []string) []string {
	completing := isCompletionInvocation(args)
	out := make([]string, 0, len(args))
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			out = append(out, args[i:]...)
			return out
		}
		name, ok := optionalAutoFlagName(arg)
		if !ok {
			out = append(out, arg)
			continue
		}
		if strings.Contains(arg, "=") {
			out = append(out, arg)
			continue
		}
		if i+1 >= len(args) {
			out = append(out, name+"="+implicitAutoValue(name))
			continue
		}
		next := args[i+1]
		switch {
		case next == "":
			out = append(out, arg)
		case isCLIFlagToken(next):
			out = append(out, name+"="+implicitAutoValue(name))
		case completing && !isKnownOptionalAutoValue(name, next):
			out = append(out, arg)
		default:
			out = append(out, name+"="+next)
			i++
		}
	}
	return out
}

func optionalAutoFlagName(arg string) (string, bool) {
	switch {
	case arg == "--icons" || strings.HasPrefix(arg, "--icons="):
		return "--icons", true
	case arg == "--color" || strings.HasPrefix(arg, "--color="):
		return "--color", true
	default:
		return "", false
	}
}

func implicitAutoValue(flagName string) string {
	if flagName == "--color" {
		return presentation.ColorAuto
	}
	return presentation.IconsAuto
}

func isKnownOptionalAutoValue(flagName, value string) bool {
	for _, item := range optionalAutoValues(flagName) {
		if item == value {
			return true
		}
	}
	return false
}

func optionalAutoValues(flagName string) []string {
	switch flagName {
	case "--icons":
		return presentation.IconModes()
	case "--color":
		return presentation.ColorModes()
	default:
		return nil
	}
}

func isCLIFlagToken(arg string) bool {
	if arg == "--" {
		return true
	}
	if len(arg) < 2 || arg[0] != '-' {
		return false
	}
	return arg != "-"
}

func isCompletionInvocation(args []string) bool {
	if len(args) == 0 {
		return false
	}
	switch args[0] {
	case cobra.ShellCompRequestCmd, cobra.ShellCompNoDescRequestCmd:
		return true
	default:
		return false
	}
}
