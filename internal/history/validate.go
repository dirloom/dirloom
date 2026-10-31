package history

import (
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
)

// Validate checks the invariants of Structural History v1 and History Result
// Schema v1. A violation is an internal invariant failure.
func (r Result) Validate() error {
	if !isObjectID(r.To) {
		return artifact.InternalError("history range has an invalid newest commit %q", r.To)
	}
	if r.From != "" && (!isObjectID(r.From) || len(r.From) != len(r.To)) {
		return artifact.InternalError("history range has an invalid lower bound %q", r.From)
	}
	if r.Limit < 1 {
		return artifact.InternalError("history limit %d is below 1", r.Limit)
	}
	if !r.End.Valid() {
		return artifact.InternalError("history end %q is unknown", r.End)
	}
	if !r.MoveDetection.Valid() {
		return artifact.InternalError("history move detection %q is unknown", r.MoveDetection)
	}
	if r.End == EndFrom && r.From == "" {
		return artifact.InternalError("history ended at from without a lower bound")
	}
	if len(r.Entries) > r.Limit {
		return artifact.InternalError("history lists %d commits above its limit %d", len(r.Entries), r.Limit)
	}
	if r.End == EndLimit && len(r.Entries) != r.Limit {
		return artifact.InternalError("history ended at limit after %d of %d commits", len(r.Entries), r.Limit)
	}
	if len(r.Entries) == 0 {
		if r.End != EndFrom || r.From != r.To {
			return artifact.InternalError("history has no entries without ending at its own lower bound")
		}
		return nil
	}
	if r.Entries[0].Commit != r.To {
		return artifact.InternalError("history starts at %s instead of %s", r.Entries[0].Commit, r.To)
	}
	last := len(r.Entries) - 1
	for i, entry := range r.Entries {
		if err := entry.validate(len(r.To)); err != nil {
			return err
		}
		if entry.Commit == r.From {
			return artifact.InternalError("history lists its exclusive lower bound %s", entry.Commit)
		}
		if i < last {
			if len(entry.Parents) == 0 || entry.Parents[0] != r.Entries[i+1].Commit {
				return artifact.InternalError("history entry %s is not followed by its first parent", entry.Commit)
			}
			if !entry.HasBase {
				return artifact.InternalError("history entry %s has no base before the end", entry.Commit)
			}
		}
	}
	tail := r.Entries[last]
	switch r.End {
	case EndRoot:
		if len(tail.Parents) != 0 {
			return artifact.InternalError("history ended at root on commit %s, which has parents", tail.Commit)
		}
	case EndShallow:
		if tail.HasBase || len(tail.Parents) == 0 {
			return artifact.InternalError("history ended at shallow on commit %s, which has a base or no parent", tail.Commit)
		}
	case EndFrom:
		if len(tail.Parents) == 0 || tail.Parents[0] != r.From {
			return artifact.InternalError("history ended at from, but commit %s does not lead to %s", tail.Commit, r.From)
		}
	}
	if r.End != EndShallow && !tail.HasBase {
		return artifact.InternalError("history entry %s has no base outside a shallow end", tail.Commit)
	}
	return nil
}

func (e Entry) validate(idLength int) error {
	if !isObjectID(e.Commit) || len(e.Commit) != idLength || !isObjectID(e.Tree) || len(e.Tree) != idLength {
		return artifact.InternalError("history entry %q has an invalid commit or tree id", e.Commit)
	}
	for _, parent := range e.Parents {
		if !isObjectID(parent) || len(parent) != idLength {
			return artifact.InternalError("history entry %s has an invalid parent id", e.Commit)
		}
	}
	if e.CommittedAt.Location() != time.UTC {
		return artifact.InternalError("history entry %s has a timestamp outside UTC", e.Commit)
	}
	if e.NodeCount < 1 {
		return artifact.InternalError("history entry %s has no root record", e.Commit)
	}
	summary := e.Summary
	if summary.Total != summary.Added+summary.Removed+summary.Changed+summary.Moved || summary.Added < 0 || summary.Removed < 0 || summary.Changed < 0 || summary.Moved < 0 {
		return artifact.InternalError("history entry %s has an inconsistent summary", e.Commit)
	}
	if !e.HasBase {
		if e.Structural || summary.Total != 0 {
			return artifact.InternalError("history entry %s has a summary but no base", e.Commit)
		}
		return nil
	}
	if e.Structural != (summary.Total > 0) {
		return artifact.InternalError("history entry %s: structural %v with %d changes", e.Commit, e.Structural, summary.Total)
	}
	return nil
}

// isObjectID reports whether id is a lowercase hex SHA-1 or SHA-256 id.
func isObjectID(id string) bool {
	if len(id) != 40 && len(id) != 64 {
		return false
	}
	for i := 0; i < len(id); i++ {
		if (id[i] < '0' || id[i] > '9') && (id[i] < 'a' || id[i] > 'f') {
			return false
		}
	}
	return true
}
