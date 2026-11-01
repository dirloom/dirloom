package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/dirloom/dirloom/internal/app"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/history"
)

// historyResultSchemaVersion is History Result Schema v1.
const historyResultSchemaVersion = 1

// historyStatusOK is the status of every successful history.
const historyStatusOK = "OK"

type historyJSONDocument struct {
	SchemaVersion int                `json:"schemaVersion"`
	Status        string             `json:"status"`
	Metadata      *historyMetadata   `json:"metadata,omitempty"`
	Range         *historyRange      `json:"range,omitempty"`
	Entries       *[]historyEntry    `json:"entries,omitempty"`
	Diagnostic    *historyDiagnostic `json:"diagnostic,omitempty"`
}

type historyMetadata struct {
	HistoryVersion            int    `json:"historyVersion"`
	Traversal                 string `json:"traversal"`
	ComparisonVersion         int    `json:"comparisonVersion"`
	IdentityProjectionVersion int    `json:"identityProjectionVersion"`
	MoveDetection             string `json:"moveDetection"`
}

type historyRange struct {
	To    string `json:"to"`
	From  string `json:"from,omitempty"`
	Limit int    `json:"limit"`
	End   string `json:"end"`
}

// historyEntry carries structural and summary only for an entry with a
// base: both are absent at a shallow end, which means unknown, never false.
type historyEntry struct {
	Commit      string          `json:"commit"`
	Tree        string          `json:"tree"`
	Parents     []string        `json:"parents"`
	CommittedAt string          `json:"committedAt"`
	Fingerprint string          `json:"fingerprint"`
	NodeCount   int             `json:"nodeCount"`
	Structural  *bool           `json:"structural,omitempty"`
	Summary     *historySummary `json:"summary,omitempty"`
}

type historySummary struct {
	Added   int `json:"added"`
	Removed int `json:"removed"`
	Changed int `json:"changed"`
	Moved   int `json:"moved"`
	Total   int `json:"total"`
}

type historyDiagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeHistoryJSON(writer io.Writer, document historyJSONDocument) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}

// historySuccessDocument encodes result; entries is result.Entries, or the
// structural entries with --structural-only. The range never changes.
func historySuccessDocument(result history.Result, entries []history.Entry) historyJSONDocument {
	encoded := make([]historyEntry, 0, len(entries))
	for _, entry := range entries {
		encoded = append(encoded, historyEntryJSON(entry))
	}
	return historyJSONDocument{
		SchemaVersion: historyResultSchemaVersion,
		Status:        historyStatusOK,
		Metadata: &historyMetadata{
			HistoryVersion:            history.Version,
			Traversal:                 history.TraversalFirstParent,
			ComparisonVersion:         comparison.Version,
			IdentityProjectionVersion: comparison.IdentityProjectionVersion,
			MoveDetection:             string(result.MoveDetection),
		},
		Range:   &historyRange{To: result.To, From: result.From, Limit: result.Limit, End: string(result.End)},
		Entries: &encoded,
	}
}

func historyEntryJSON(entry history.Entry) historyEntry {
	parents := append([]string{}, entry.Parents...)
	out := historyEntry{
		Commit:      entry.Commit,
		Tree:        entry.Tree,
		Parents:     parents,
		CommittedAt: entry.CommittedAt.UTC().Format(time.RFC3339),
		Fingerprint: entry.Fingerprint.String(),
		NodeCount:   entry.NodeCount,
	}
	if entry.HasBase {
		structural := entry.Structural
		out.Structural = &structural
		out.Summary = &historySummary{
			Added:   entry.Summary.Added,
			Removed: entry.Summary.Removed,
			Changed: entry.Summary.Changed,
			Moved:   entry.Summary.Moved,
			Total:   entry.Summary.Total,
		}
	}
	return out
}

func historyFailureDocument(kind app.VerifyFailureKind, err error) historyJSONDocument {
	code, message := app.HistoryDiagnostic(err)
	return historyJSONDocument{
		SchemaVersion: historyResultSchemaVersion,
		Status:        string(kind),
		Diagnostic:    &historyDiagnostic{Code: code, Message: message},
	}
}

// historyShortID is the commit id length of the text output.
const historyShortID = 12

// writeHistoryText renders the human projection of a history. It emits no
// ANSI. entries is what --structural-only leaves; the header always counts
// the commits examined.
func writeHistoryText(writer io.Writer, result history.Result, entries []history.Entry, structuralOnly bool) error {
	var builder strings.Builder
	fmt.Fprintf(&builder, "Structural History (first parent, %s examined", countCommits(len(result.Entries)))
	if structuralOnly {
		fmt.Fprintf(&builder, ", %d shown", len(entries))
	}
	fmt.Fprintf(&builder, ", end: %s)\n", result.End)
	if len(entries) == 0 {
		builder.WriteString("\nNo commits to show.\n")
		return writeAll(writer, []byte(builder.String()))
	}
	builder.WriteString("\n")
	for _, entry := range entries {
		builder.WriteString(historyLine(entry))
		builder.WriteString("\n")
	}
	builder.WriteString("\nChanged paths of a commit: dirloom diff git:<commit>^ git:<commit>\n")
	return writeAll(writer, []byte(builder.String()))
}

func historyLine(entry history.Entry) string {
	parts := []string{entry.Commit[:historyShortID], entry.CommittedAt.UTC().Format(time.RFC3339)}
	switch {
	case !entry.HasBase:
		parts = append(parts, "structure unknown (shallow boundary)")
	case !entry.Structural:
		parts = append(parts, "no structural change")
	default:
		summary := entry.Summary
		parts = append(parts, fmt.Sprintf("+%d -%d ~%d >%d", summary.Added, summary.Removed, summary.Changed, summary.Moved))
	}
	switch {
	case len(entry.Parents) == 0:
		parts = append(parts, "root")
	case len(entry.Parents) > 1:
		parts = append(parts, fmt.Sprintf("merge (%d parents)", len(entry.Parents)))
	}
	return strings.Join(parts, "  ")
}

func countCommits(n int) string {
	if n == 1 {
		return "1 commit"
	}
	return fmt.Sprintf("%d commits", n)
}
