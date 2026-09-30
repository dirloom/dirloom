package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/dirloom/dirloom/internal/app"
	"github.com/dirloom/dirloom/internal/comparison"
)

const diffResultSchemaVersion = 1

type diffJSONDocument struct {
	SchemaVersion int             `json:"schemaVersion"`
	Status        string          `json:"status"`
	Metadata      *diffMetadata   `json:"metadata,omitempty"`
	Summary       *diffSummary    `json:"summary,omitempty"`
	Changes       *[]diffChange   `json:"changes,omitempty"`
	Diagnostic    *diffDiagnostic `json:"diagnostic,omitempty"`
}

type diffMetadata struct {
	ComparisonVersion         int           `json:"comparisonVersion"`
	IdentityProjectionVersion int           `json:"identityProjectionVersion"`
	A                         diffSourceRef `json:"a"`
	B                         diffSourceRef `json:"b"`
}

type diffSourceRef struct {
	Kind      string `json:"kind"`
	NodeCount int    `json:"nodeCount"`
}

type diffSummary struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Changed int `json:"changed"`
	Total   int `json:"total"`
}

type diffChange struct {
	Path   string         `json:"path"`
	Op     string         `json:"op"`
	Before *diffNodeState `json:"before,omitempty"`
	After  *diffNodeState `json:"after,omitempty"`
}

// diffNodeState carries target with explicit presence: a non-nil pointer to an
// empty string still encodes as "target": "".
type diffNodeState struct {
	Kind   string  `json:"kind"`
	Target *string `json:"target,omitempty"`
}

type diffDiagnostic struct {
	Source  string `json:"source,omitempty"`
	Code    string `json:"code"`
	Message string `json:"message"`
}

// writeDiffText renders the human projection of a validated diff. It emits no
// ANSI and no presentation decoration. "<none>" for an absent target is
// presentation only and never part of the model.
func writeDiffText(writer io.Writer, diff comparison.StructuralDiff) error {
	if len(diff.Changes) == 0 {
		_, err := io.WriteString(writer, "No structural differences.\n")
		return err
	}
	var builder strings.Builder
	builder.WriteString("Structural Diff\n\n")
	fmt.Fprintf(&builder, "Summary: %d added, %d removed, %d changed, %d total\n",
		diff.Summary.Added, diff.Summary.Removed, diff.Summary.Changed, diff.Summary.Total)
	writeDiffSection(&builder, "ADDED", "+", diff.Changes, comparison.OpAdded)
	writeDiffSection(&builder, "REMOVED", "-", diff.Changes, comparison.OpRemoved)
	writeDiffSection(&builder, "CHANGED", "~", diff.Changes, comparison.OpChanged)
	return writeAll(writer, []byte(builder.String()))
}

func writeDiffSection(builder *strings.Builder, title, prefix string, changes []comparison.Change, op comparison.Operation) {
	first := true
	for _, change := range changes {
		if change.Op != op {
			continue
		}
		if first {
			builder.WriteString("\n")
			builder.WriteString(title)
			builder.WriteString("\n")
			first = false
		}
		builder.WriteString("  ")
		builder.WriteString(prefix)
		builder.WriteString(" ")
		builder.WriteString(change.Path.String())
		if change.Op == comparison.OpChanged {
			builder.WriteString(" (")
			builder.WriteString(changedAttributes(change.Before, change.After))
			builder.WriteString(")")
		}
		builder.WriteString("\n")
	}
}

// changedAttributes lists only the attributes that differ between before and
// after: kind, and target presence or value.
func changedAttributes(before, after *comparison.NodeState) string {
	var parts []string
	if before.Kind != after.Kind {
		parts = append(parts, fmt.Sprintf("kind: %s -> %s", before.Kind, after.Kind))
	}
	if !targetPointerEqual(before.Target, after.Target) {
		parts = append(parts, fmt.Sprintf("target: %s -> %s", targetOrNone(before.Target), targetOrNone(after.Target)))
	}
	return strings.Join(parts, ", ")
}

func targetPointerEqual(a, b *string) bool {
	if (a == nil) != (b == nil) {
		return false
	}
	if a == nil {
		return true
	}
	return *a == *b
}

func targetOrNone(target *string) string {
	if target == nil {
		return "<none>"
	}
	return *target
}

func writeDiffJSON(writer io.Writer, document diffJSONDocument) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}

func diffSuccessDocument(diff comparison.StructuralDiff) diffJSONDocument {
	changes := make([]diffChange, 0, len(diff.Changes))
	for _, change := range diff.Changes {
		changes = append(changes, diffChangeJSON(change))
	}
	return diffJSONDocument{
		SchemaVersion: diffResultSchemaVersion,
		Status:        string(app.DiffStatusOf(diff)),
		Metadata: &diffMetadata{
			ComparisonVersion:         diff.Metadata.ComparisonVersion,
			IdentityProjectionVersion: diff.Metadata.IdentityProjectionVersion,
			A:                         diffSourceRef{Kind: string(diff.Metadata.A.Kind), NodeCount: diff.Metadata.A.NodeCount},
			B:                         diffSourceRef{Kind: string(diff.Metadata.B.Kind), NodeCount: diff.Metadata.B.NodeCount},
		},
		Summary: &diffSummary{
			Added:   diff.Summary.Added,
			Removed: diff.Summary.Removed,
			Changed: diff.Summary.Changed,
			Total:   diff.Summary.Total,
		},
		Changes: &changes,
	}
}

func diffChangeJSON(change comparison.Change) diffChange {
	out := diffChange{Path: change.Path.String(), Op: string(change.Op)}
	if change.Before != nil {
		out.Before = diffNodeStateJSON(*change.Before)
	}
	if change.After != nil {
		out.After = diffNodeStateJSON(*change.After)
	}
	return out
}

func diffNodeStateJSON(state comparison.NodeState) *diffNodeState {
	out := &diffNodeState{Kind: string(state.Kind)}
	if state.Target != nil {
		target := *state.Target
		out.Target = &target
	}
	return out
}

func diffFailureDocument(kind app.VerifyFailureKind, err error) diffJSONDocument {
	code, message := app.DiffDiagnostic(err)
	diagnostic := &diffDiagnostic{Code: code, Message: message}
	if side, ok := comparison.SideOf(err); ok {
		diagnostic.Source = string(side)
	}
	return diffJSONDocument{
		SchemaVersion: diffResultSchemaVersion,
		Status:        string(kind),
		Diagnostic:    diagnostic,
	}
}
