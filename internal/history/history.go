// Package history walks the first-parent chain of a Git revision and
// describes the structure of each commit: its fingerprint and the
// structural changes against its first parent (Structural History v1). It
// reuses the Git source, Identity Projection v1, Fingerprint v1 and the
// comparison engine; it has no structural model of its own.
package history

import (
	"context"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/comparison"
	"github.com/dirloom/dirloom/internal/gitsource"
	"github.com/dirloom/dirloom/internal/identity"
	"github.com/dirloom/dirloom/internal/source"
	"github.com/dirloom/dirloom/internal/tree"
)

const (
	// Version is the Structural History contract version.
	Version = 1
	// TraversalFirstParent is the only traversal of Structural History v1.
	TraversalFirstParent = "first-parent"
	// DefaultLimit is the number of commits examined when no limit is given.
	DefaultLimit = 20
	// DefaultRevision is the newest commit when no revision is given.
	DefaultRevision = "HEAD"
	// rootName labels the root of every observed tree. Revision text never
	// reaches a result.
	rootName = "git"
)

// End says why a walk stopped.
type End string

const (
	// EndFrom: the next commit was the exclusive lower bound.
	EndFrom End = "from"
	// EndLimit: the requested number of commits was examined.
	EndLimit End = "limit"
	// EndRoot: the last listed commit has no parent.
	EndRoot End = "root"
	// EndShallow: the last listed commit is a recorded shallow boundary whose
	// first parent is absent.
	EndShallow End = "shallow"
)

// Valid reports whether e is one of the ends.
func (e End) Valid() bool {
	return e == EndFrom || e == EndLimit || e == EndRoot || e == EndShallow
}

// Request is one history walk. Revision and From must already satisfy
// gitsource.ValidateRevision; an empty Revision means DefaultRevision and an
// empty MoveDetection means structural-v1. Dir is where Git discovers the
// repository; empty means the process working directory.
type Request struct {
	Revision      string
	From          string
	Limit         int
	MoveDetection comparison.MoveDetection
	Dir           string
}

// Entry describes one listed commit. Structural and Summary are meaningful
// only when HasBase is true; an entry without a base is the last one of a
// walk that ended at a shallow boundary.
type Entry struct {
	Commit      string
	Tree        string
	Parents     []string
	CommittedAt time.Time
	Fingerprint identity.Fingerprint
	NodeCount   int
	HasBase     bool
	Structural  bool
	Summary     comparison.Summary
}

// Result is a validated history: the resolved range, why the walk ended,
// and the listed commits, newest first.
type Result struct {
	To            string
	From          string
	Limit         int
	End           End
	MoveDetection comparison.MoveDetection
	Entries       []Entry
}

// StructuralEntries returns the entries known to change the structure, the
// view `--structural-only` shows: entries with structural false and entries
// without a base are left out. The result's range is unaffected.
func (r Result) StructuralEntries() []Entry {
	out := make([]Entry, 0, len(r.Entries))
	for _, entry := range r.Entries {
		if entry.HasBase && entry.Structural {
			out = append(out, entry)
		}
	}
	return out
}

// Walk runs one history through a single Git session. It is transactional:
// it returns a validated result, or an error and no entries.
func Walk(ctx context.Context, request Request) (Result, error) {
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	if request.Limit < 1 {
		return Result{}, artifact.InternalError("history limit %d is below 1", request.Limit)
	}
	if request.Revision == "" {
		request.Revision = DefaultRevision
	}
	if request.MoveDetection == "" {
		request.MoveDetection = comparison.MoveDetectionStructuralV1
	}
	if !request.MoveDetection.Valid() {
		return Result{}, artifact.InternalError("unsupported move detection %q", request.MoveDetection)
	}
	session, err := gitsource.OpenSession(ctx, request.Dir)
	if err != nil {
		return Result{}, err
	}
	result, walkErr := walk(ctx, session, request)
	closeErr := session.Close()
	if walkErr != nil {
		return Result{}, walkErr
	}
	if closeErr != nil {
		return Result{}, closeErr
	}
	if err := result.Validate(); err != nil {
		return Result{}, err
	}
	return result, nil
}

func walk(ctx context.Context, session *gitsource.Session, request Request) (Result, error) {
	to, err := session.ResolveCommit(ctx, request.Revision)
	if err != nil {
		return Result{}, err
	}
	from := ""
	if request.From != "" {
		if from, err = session.ResolveCommit(ctx, request.From); err != nil {
			return Result{}, err
		}
	}
	boundaries, err := session.ShallowBoundaries(ctx)
	if err != nil {
		return Result{}, err
	}
	result := Result{To: to, From: from, Limit: request.Limit, MoveDetection: request.MoveDetection, Entries: []Entry{}}
	if to == from {
		result.End = EndFrom
		return result, nil
	}

	commit, err := session.ReadCommit(to)
	if err != nil {
		return Result{}, err
	}
	current, err := observe(ctx, session, commit.Tree)
	if err != nil {
		return Result{}, err
	}
	for {
		if err := ctx.Err(); err != nil {
			return Result{}, err
		}
		entry := Entry{
			Commit:      commit.ID,
			Tree:        commit.Tree,
			Parents:     commit.Parents,
			CommittedAt: commit.CommittedAt,
			Fingerprint: current.fingerprint,
			NodeCount:   len(current.records),
		}
		if len(commit.Parents) == 0 {
			base, err := observe(ctx, session, session.Format().EmptyTree)
			if err != nil {
				return Result{}, err
			}
			if err := compareWithBase(&entry, base, "", current, request.MoveDetection); err != nil {
				return Result{}, err
			}
			result.Entries = append(result.Entries, entry)
			result.End = EndRoot
			break
		}
		parentID := commit.Parents[0]
		parent, err := session.ReadCommit(parentID)
		if err != nil {
			if _, boundary := boundaries[commit.ID]; boundary && gitsource.IsMissing(err) {
				result.Entries = append(result.Entries, entry)
				result.End = EndShallow
				break
			}
			return Result{}, err
		}
		base, err := observe(ctx, session, parent.Tree)
		if err != nil {
			return Result{}, err
		}
		if err := compareWithBase(&entry, base, parent.ID, current, request.MoveDetection); err != nil {
			return Result{}, err
		}
		result.Entries = append(result.Entries, entry)
		if parentID == from {
			result.End = EndFrom
			break
		}
		if len(result.Entries) == request.Limit {
			result.End = EndLimit
			break
		}
		commit, current = parent, base
	}
	if from != "" && (result.End == EndRoot || result.End == EndShallow) {
		return Result{}, artifact.New(artifact.CodeHistoryFromNotReached,
			"the walk reached "+endDescription(result.End)+" without meeting --from: it is not on the first-parent chain of the revision", false)
	}
	return result, nil
}

func endDescription(end End) string {
	if end == EndShallow {
		return "a shallow boundary"
	}
	return "a root commit"
}

// observation is one tree's Identity Projection v1 and fingerprint. A walk
// keeps at most two of them.
type observation struct {
	tree        string
	records     []identity.Record
	fingerprint identity.Fingerprint
}

func observe(ctx context.Context, session *gitsource.Session, treeID string) (observation, error) {
	art, err := source.ObserveGitTree(ctx, session, treeID, tree.ScanOptions{RootName: rootName})
	if err != nil {
		return observation{}, err
	}
	records, err := identity.ProjectV1(*art)
	if err != nil {
		return observation{}, err
	}
	fingerprint, err := identity.FingerprintRecords(records)
	if err != nil {
		return observation{}, err
	}
	return observation{tree: treeID, records: records, fingerprint: fingerprint}, nil
}

// compareWithBase fills the entry's summary against base (source A) and
// checks that the structural flag agrees with the fingerprints.
func compareWithBase(entry *Entry, base observation, baseCommit string, current observation, strategy comparison.MoveDetection) error {
	diff, err := comparison.CompareRecords(base.records, current.records,
		comparison.SourceRef{Kind: source.KindGit, Tree: base.tree, Commit: baseCommit},
		comparison.SourceRef{Kind: source.KindGit, Tree: current.tree, Commit: entry.Commit},
		comparison.Options{MoveDetection: strategy})
	if err != nil {
		return err
	}
	entry.HasBase = true
	entry.Summary = diff.Summary
	entry.Structural = diff.Summary.Total > 0
	if entry.Structural != (base.fingerprint != current.fingerprint) {
		return artifact.InternalError("commit %s: structural %v disagrees with its fingerprints", entry.Commit, entry.Structural)
	}
	return nil
}
