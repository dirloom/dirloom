package cli

import (
	"fmt"
	"io"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

const helpTopicsMetaName = "topics"

func newHelpCommand() *cobra.Command {
	command := &cobra.Command{
		Use:                   "help [command | topic]",
		Short:                 "Help about commands and conceptual topics",
		Long:                  "Help about any command or compiled conceptual topic.\n\nRun 'dirloom help topics' to list conceptual topics such as icons, formats, and configuration.",
		DisableFlagsInUseLine: true,
		SilenceErrors:         true,
		SilenceUsage:          true,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runHelp(cmd, args)
		},
		ValidArgsFunction: completeHelpTargets,
	}
	command.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return &usageError{err: err}
	})
	return command
}

func runHelp(cmd *cobra.Command, args []string) error {
	root := cmd.Root()
	out := cmd.OutOrStdout()
	if len(args) == 0 {
		return root.Help()
	}
	if len(args) == 1 && args[0] == helpTopicsMetaName {
		return writeHelpTopicList(out)
	}
	target, _, err := root.Find(args)
	if err == nil && target != nil && target != root {
		return target.Help()
	}
	if len(args) != 1 {
		return unknownHelpTarget(root, strings.Join(args, " "))
	}
	topic, ok := lookupHelpTopic(args[0])
	if !ok {
		return unknownHelpTarget(root, args[0])
	}
	return writeHelpTopic(out, topic)
}

func writeHelpTopic(writer io.Writer, topic helpTopic) error {
	var builder strings.Builder
	builder.WriteString(topic.Body)
	if !strings.HasSuffix(topic.Body, "\n") {
		builder.WriteByte('\n')
	}
	if len(topic.SeeAlso) > 0 {
		builder.WriteString("\nSee also:\n")
		for _, name := range topic.SeeAlso {
			builder.WriteString("  dirloom help ")
			builder.WriteString(name)
			builder.WriteByte('\n')
		}
	}
	return writeAll(writer, []byte(builder.String()))
}

func writeHelpTopicList(writer io.Writer) error {
	var builder strings.Builder
	builder.WriteString("Help topics:\n\n")
	width := 0
	for _, topic := range helpTopics {
		if len(topic.Name) > width {
			width = len(topic.Name)
		}
	}
	for _, topic := range helpTopics {
		fmt.Fprintf(&builder, "  %-*s  %s\n", width, topic.Name, topic.Summary)
	}
	builder.WriteString("\nRun 'dirloom help <topic>' for details.\n")
	return writeAll(writer, []byte(builder.String()))
}

func unknownHelpTarget(root *cobra.Command, name string) error {
	return &usageError{
		err:         fmt.Errorf("unknown help topic or command %q", name),
		suggestions: suggestNames(name, helpSuggestionCandidates(root)),
		hint:        "Run 'dirloom help topics' to list available topics.",
	}
}

func helpSuggestionCandidates(root *cobra.Command) []string {
	candidates := []string{helpTopicsMetaName}
	candidates = append(candidates, helpTopicNames()...)
	for _, topic := range helpTopics {
		candidates = append(candidates, topic.Aliases...)
	}
	if root == nil {
		return candidates
	}
	for _, command := range root.Commands() {
		if !command.IsAvailableCommand() {
			continue
		}
		candidates = append(candidates, command.Name())
		candidates = append(candidates, command.Aliases...)
	}
	return candidates
}

func completeHelpTargets(cmd *cobra.Command, args []string, _ string) ([]string, cobra.ShellCompDirective) {
	root := cmd.Root()
	if len(args) == 0 {
		names := []string{helpTopicsMetaName}
		for _, topic := range helpTopics {
			names = append(names, topic.Name)
			names = append(names, topic.Aliases...)
		}
		for _, command := range root.Commands() {
			if !command.IsAvailableCommand() {
				continue
			}
			names = append(names, command.Name())
		}
		return uniqueSorted(names), cobra.ShellCompDirectiveNoFileComp
	}
	target, leftover, err := root.Find(args)
	if err != nil || target == nil || target == root || len(leftover) > 0 {
		return nil, cobra.ShellCompDirectiveNoFileComp
	}
	var names []string
	for _, command := range target.Commands() {
		if !command.IsAvailableCommand() {
			continue
		}
		names = append(names, command.Name())
	}
	return uniqueSorted(names), cobra.ShellCompDirectiveNoFileComp
}

func uniqueSorted(values []string) []string {
	if len(values) == 0 {
		return nil
	}
	sort.Strings(values)
	out := values[:0]
	previous := ""
	for i, value := range values {
		if value == "" || (i > 0 && value == previous) {
			continue
		}
		out = append(out, value)
		previous = value
	}
	return out
}

func suggestNames(input string, candidates []string) []string {
	best := -1
	var matches []string
	seen := make(map[string]struct{}, len(candidates))
	for _, candidate := range candidates {
		if candidate == "" || candidate == input {
			continue
		}
		if _, ok := seen[candidate]; ok {
			continue
		}
		seen[candidate] = struct{}{}
		distance := nameDistance(input, candidate)
		maxLen := len(input)
		if len(candidate) > maxLen {
			maxLen = len(candidate)
		}
		if distance > 2 || (maxLen > 0 && distance > maxLen/2) {
			continue
		}
		if best < 0 || distance < best {
			best = distance
			matches = []string{candidate}
			continue
		}
		if distance == best {
			matches = append(matches, candidate)
		}
	}
	sort.Strings(matches)
	return matches
}

func nameDistance(input, candidate string) int {
	distance := levenshtein(input, candidate)
	if len(input) >= 3 && strings.HasPrefix(candidate, input) {
		extra := len(candidate) - len(input)
		if extra < distance {
			return extra
		}
	}
	return distance
}

func levenshtein(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return len(b)
	}
	if b == "" {
		return len(a)
	}
	previous := make([]int, len(b)+1)
	current := make([]int, len(b)+1)
	for j := range previous {
		previous[j] = j
	}
	for i := 1; i <= len(a); i++ {
		current[0] = i
		for j := 1; j <= len(b); j++ {
			cost := 1
			if a[i-1] == b[j-1] {
				cost = 0
			}
			deletion := previous[j] + 1
			insertion := current[j-1] + 1
			substitution := previous[j-1] + cost
			current[j] = deletion
			if insertion < current[j] {
				current[j] = insertion
			}
			if substitution < current[j] {
				current[j] = substitution
			}
		}
		previous, current = current, previous
	}
	return previous[len(b)]
}

func invalidEnumeratedFlag(flag, value string, allowed []string, topic string) *usageError {
	return &usageError{
		err:       fmt.Errorf("invalid value %q for --%s", value, flag),
		values:    append([]string(nil), allowed...),
		helpTopic: topic,
	}
}

func requireEnumeratedFlag(flag, value string, allowed []string, topic string) error {
	if value == "" {
		return &usageError{err: fmt.Errorf("--%s requires a non-empty value", flag)}
	}
	for _, item := range allowed {
		if item == value {
			return nil
		}
	}
	return invalidEnumeratedFlag(flag, value, allowed, topic)
}

func containsANSI(text string) bool {
	return strings.ContainsRune(text, '\x1b')
}

func containsNerdGlyph(text string) bool {
	for _, r := range text {
		if r >= 0xE000 && r <= 0xF8FF {
			return true
		}
	}
	return false
}
