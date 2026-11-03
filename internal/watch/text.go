package watch

import (
	"fmt"
	"io"
	"strings"
	"unicode"

	"github.com/dirloom/dirloom/internal/comparison"
)

// TextSink renders a run for a person; it is not a contract (Watch Event
// Schema v1 is). Events go to out, with the diff text symbols for changes;
// diagnostics go to errOut, "Error: " when fatal and "Warning: " otherwise.
// It writes no ANSI sequence, and control characters in paths, targets and
// messages are escaped.
type TextSink struct {
	out, errOut io.Writer
}

// NewTextSink returns a sink writing events to out and diagnostics to
// errOut.
func NewTextSink(out, errOut io.Writer) *TextSink { return &TextSink{out: out, errOut: errOut} }

// Publish renders events; diagnostics are written before the events of the
// same batch, so that a fatal error precedes the final line.
func (s *TextSink) Publish(events []Event) error {
	var out, diagnostics strings.Builder
	for _, ev := range events {
		switch {
		case ev.Kind == KindStarted:
			fmt.Fprintf(&out, "Watching structure (%s, %s): %s, %s\n",
				ev.Metadata.Stability, ev.Metadata.Backend, nodeCount(ev.NodeCount), ev.Fingerprint)
		case ev.Kind.IsNode():
			fmt.Fprintf(&out, "#%d  %s\n", ev.Generation, changeText(ev.Change))
		case ev.Kind == KindStateChanged:
			sum := ev.Summary
			fmt.Fprintf(&out, "#%d  = %s, %s  (+%d -%d ~%d >%d)\n", ev.Generation, nodeCount(ev.NodeCount), ev.Fingerprint,
				sum.Added, sum.Removed, sum.Changed, sum.Moved)
		case ev.Kind == KindRescanStarted:
			fmt.Fprintf(&out, "rescan (%s)\n", ev.Rescan)
		case ev.Kind == KindRescanCompleted:
			out.WriteString("rescan completed\n")
		case ev.Kind == KindError:
			label := "Warning"
			if ev.Fatal {
				label = "Error"
			}
			fmt.Fprintf(&diagnostics, "%s: %s (%s)\n", label, safeText(ev.Diagnostic.Message), safeText(ev.Diagnostic.Code))
		case ev.Kind == KindStopped && ev.HasGeneration:
			fmt.Fprintf(&out, "Stopped at #%d.\n", ev.Generation)
		case ev.Kind == KindStopped:
			out.WriteString("Stopped before the first state.\n")
		}
	}
	if diagnostics.Len() > 0 {
		if _, err := io.WriteString(s.errOut, diagnostics.String()); err != nil {
			return err
		}
	}
	if out.Len() > 0 {
		if _, err := io.WriteString(s.out, out.String()); err != nil {
			return err
		}
	}
	return nil
}

// changeText is one change as dirloom diff prints it, after its symbol.
func changeText(c comparison.Change) string {
	switch c.Op {
	case comparison.OpAdded:
		return "+ " + safeText(c.Path.String())
	case comparison.OpRemoved:
		return "- " + safeText(c.Path.String())
	case comparison.OpChanged:
		return "~ " + safeText(c.Path.String()) + " (" + changedAttributes(c.Before, c.After) + ")"
	default:
		return fmt.Sprintf("> %s -> %s (%s, %s)", safeText(c.From.String()), safeText(c.Path.String()), c.Evidence, nodeCount(c.Nodes))
	}
}

// changedAttributes lists the attributes that differ, as dirloom diff does:
// kind, and target presence or value.
func changedAttributes(before, after *comparison.NodeState) string {
	var parts []string
	if before.Kind != after.Kind {
		parts = append(parts, fmt.Sprintf("kind: %s -> %s", before.Kind, after.Kind))
	}
	if targetOf(before) != targetOf(after) {
		parts = append(parts, fmt.Sprintf("target: %s -> %s", targetOf(before), targetOf(after)))
	}
	return strings.Join(parts, ", ")
}

// targetOf is the displayed target; "<none>" for an absent one is
// presentation only.
func targetOf(state *comparison.NodeState) string {
	if state.Target == nil {
		return "<none>"
	}
	return safeText(*state.Target)
}

func nodeCount(n int) string {
	if n == 1 {
		return "1 node"
	}
	return fmt.Sprintf("%d nodes", n)
}

// safeText escapes what a terminal would interpret or hide: control
// characters, line and paragraph separators, and bidirectional controls.
func safeText(value string) string {
	var out strings.Builder
	for _, r := range value {
		switch r {
		case '\t':
			out.WriteString(`\t`)
		case '\n':
			out.WriteString(`\n`)
		case '\r':
			out.WriteString(`\r`)
		default:
			if unicode.IsControl(r) || unicode.In(r, unicode.Zl, unicode.Zp) || bidiControl(r) {
				fmt.Fprintf(&out, `\u{%04X}`, r)
			} else {
				out.WriteRune(r)
			}
		}
	}
	return out.String()
}

func bidiControl(r rune) bool {
	switch r {
	case 0x061c, 0x200e, 0x200f, 0x202a, 0x202b, 0x202c, 0x202d, 0x202e, 0x2066, 0x2067, 0x2068, 0x2069:
		return true
	}
	return false
}
