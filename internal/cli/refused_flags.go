package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// refusedFlag is a flag that other commands take and that a command refuses
// with a reason. It is declared, so the refusal names it instead of "unknown
// flag", but hidden from help and completion: help lists only usable flags.
type refusedFlag struct {
	name      string
	shorthand string
	boolean   bool
	reason    string
}

// Reasons shared by several commands.
const (
	reasonNoPresentation   = "its output has no presentation options"
	reasonStdoutOnly       = "it writes to stdout only"
	reasonCaptureSemantics = "snapshot capture semantics are authoritative"
	reasonNoConfiguration  = "it reads no configuration file"
)

// sourceFlags refuses the root's persistent configuration-source flags.
// Declaring them on the command shadows the inherited ones, so its help no
// longer lists them under "Global Flags".
func sourceFlags(reason string) []refusedFlag {
	return []refusedFlag{
		{name: "config", reason: reason},
		{name: "no-user-config", boolean: true, reason: reason},
		{name: "no-config", boolean: true, reason: reason},
	}
}

// scopeFlags refuses the structural capture flags of the tree output,
// fingerprint, snapshot and watch.
func scopeFlags(reason string) []refusedFlag {
	return []refusedFlag{
		{name: "preset", reason: reason},
		{name: "depth", shorthand: "d", reason: reason},
		{name: "dirs-only", boolean: true, reason: reason},
		{name: "hidden", boolean: true, reason: reason},
		{name: "ignore", reason: reason},
		{name: "no-default-ignore", boolean: true, reason: reason},
		{name: "no-gitignore", boolean: true, reason: reason},
	}
}

// presentationFlags refuses the terminal presentation flags.
func presentationFlags() []refusedFlag {
	return []refusedFlag{
		{name: "color", reason: reasonNoPresentation},
		{name: "icons", reason: reasonNoPresentation},
		{name: "theme", reason: reasonNoPresentation},
		{name: "style", reason: reasonNoPresentation},
	}
}

// destinationFlags refuses the file and clipboard destinations.
func destinationFlags(reason string) []refusedFlag {
	return []refusedFlag{
		{name: "output", shorthand: "o", reason: reason},
		{name: "copy", boolean: true, reason: reason},
	}
}

func joinRefusedFlags(groups ...[]refusedFlag) []refusedFlag {
	var all []refusedFlag
	for _, group := range groups {
		all = append(all, group...)
	}
	return all
}

// registerRefusedFlags declares the refused flags of a command, hidden.
func registerRefusedFlags(command *cobra.Command, refused []refusedFlag) {
	flags := command.Flags()
	for _, flag := range refused {
		usage := "not supported by " + command.Name()
		if flag.boolean {
			flags.BoolP(flag.name, flag.shorthand, false, usage)
		} else {
			flags.StringP(flag.name, flag.shorthand, "", usage)
		}
		_ = flags.MarkHidden(flag.name)
	}
}

// rejectRefusedFlags returns the usage error of the first refused flag on
// the command line, in declaration order. subject names the command in the
// message, as in "--depth is not supported by verify: <reason>".
func rejectRefusedFlags(command *cobra.Command, subject string, refused []refusedFlag) error {
	for _, flag := range refused {
		if command.Flags().Changed(flag.name) {
			return &usageError{err: fmt.Errorf("--%s is not supported by %s: %s", flag.name, subject, flag.reason)}
		}
	}
	return nil
}
