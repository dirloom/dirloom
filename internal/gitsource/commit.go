package gitsource

import (
	"bytes"
	"context"
	"time"

	"github.com/dirloom/dirloom/internal/artifact"
)

// commitObject is the batch kind of a commit (Structural History v1).
var commitObject = objectKind{typ: "commit", limit: 1 << 20, label: "commit object"}

// maxCommitTimestamp is 9999-12-31T23:59:59Z, the last second RFC 3339
// writes with a four-digit year.
const maxCommitTimestamp = 253402300799

// Commit is the part of a commit object a structural history reads. Ids are
// lowercase hex; CommittedAt is in UTC.
type Commit struct {
	ID          string
	Tree        string
	Parents     []string
	CommittedAt time.Time
}

// ParseCommit parses a raw commit object strictly: `tree`, then `parent`
// headers, then other headers with exactly one `committer`, up to the first
// empty line. Identities and the message are skipped, never kept.
func ParseCommit(id string, data []byte, format ObjectFormat) (Commit, error) {
	const (
		expectTree = iota
		expectParents
		expectOther
	)
	commit := Commit{ID: id}
	phase := expectTree
	committers := 0
	previous := ""
	for rest := data; len(rest) > 0; {
		var line []byte
		if end := bytes.IndexByte(rest, '\n'); end >= 0 {
			line, rest = rest[:end], rest[end+1:]
		} else {
			line, rest = rest, nil
		}
		if len(line) == 0 {
			break
		}
		if line[0] == ' ' {
			// Continuation of a multi-line header such as gpgsig or mergetag.
			if previous == "" || previous == "tree" || previous == "parent" || previous == "committer" {
				return Commit{}, commitInvalid("commit %s has a continuation line after %s", id, quoteToken(previous))
			}
			continue
		}
		name, value, ok := bytes.Cut(line, []byte{' '})
		if !ok || len(name) == 0 {
			return Commit{}, commitInvalid("commit %s has a malformed header", id)
		}
		switch string(name) {
		case "tree":
			if phase != expectTree {
				return Commit{}, commitInvalid("commit %s has a misplaced tree header", id)
			}
			if !format.isObjectID(string(value)) {
				return Commit{}, commitInvalid("commit %s has an invalid tree id", id)
			}
			commit.Tree = string(value)
			phase = expectParents
		case "parent":
			if phase != expectParents {
				return Commit{}, commitInvalid("commit %s has a misplaced parent header", id)
			}
			if !format.isObjectID(string(value)) {
				return Commit{}, commitInvalid("commit %s has an invalid parent id", id)
			}
			commit.Parents = append(commit.Parents, string(value))
		default:
			if phase == expectTree {
				return Commit{}, commitInvalid("commit %s does not start with a tree header", id)
			}
			phase = expectOther
			if string(name) == "committer" {
				committers++
				committedAt, ok := parseCommitter(value)
				if !ok {
					return Commit{}, commitInvalid("commit %s has an invalid committer timestamp or zone", id)
				}
				commit.CommittedAt = committedAt
			}
		}
		previous = string(name)
	}
	if phase == expectTree {
		return Commit{}, commitInvalid("commit %s has no tree header", id)
	}
	if committers != 1 {
		return Commit{}, commitInvalid("commit %s has %d committer headers, expected one", id, committers)
	}
	return commit, nil
}

// parseCommitter reads `<identity> <timestamp> <zone>` from the right. The
// identity is skipped; the zone is validated and dropped.
func parseCommitter(value []byte) (time.Time, bool) {
	zoneStart := bytes.LastIndexByte(value, ' ')
	if zoneStart <= 0 {
		return time.Time{}, false
	}
	zone := value[zoneStart+1:]
	timestampStart := bytes.LastIndexByte(value[:zoneStart], ' ')
	if timestampStart <= 0 {
		return time.Time{}, false
	}
	seconds, ok := parseTimestamp(value[timestampStart+1 : zoneStart])
	if !ok || !validZone(zone) {
		return time.Time{}, false
	}
	return time.Unix(seconds, 0).UTC(), true
}

// parseTimestamp accepts an unsigned decimal without a leading zero (except
// "0"), at most maxCommitTimestamp.
func parseTimestamp(text []byte) (int64, bool) {
	if len(text) == 0 || len(text) > 12 || (len(text) > 1 && text[0] == '0') {
		return 0, false
	}
	var value int64
	for _, c := range text {
		if c < '0' || c > '9' {
			return 0, false
		}
		value = value*10 + int64(c-'0')
	}
	return value, value <= maxCommitTimestamp
}

// validZone accepts `+HHMM` or `-HHMM` with hours 00–14 and minutes 00–59.
func validZone(zone []byte) bool {
	if len(zone) != 5 || (zone[0] != '+' && zone[0] != '-') {
		return false
	}
	for _, c := range zone[1:] {
		if c < '0' || c > '9' {
			return false
		}
	}
	hours := int(zone[1]-'0')*10 + int(zone[2]-'0')
	minutes := int(zone[3]-'0')*10 + int(zone[4]-'0')
	return hours <= 14 && minutes <= 59
}

// ReadCommit reads and parses one commit object through the session's batch
// process. An absent object satisfies IsMissing.
func (s *Session) ReadCommit(id string) (Commit, error) {
	batch, err := s.objects()
	if err != nil {
		return Commit{}, err
	}
	data, err := batch.read(id, commitObject)
	if err != nil {
		return Commit{}, err
	}
	return ParseCommit(id, data, s.format)
}

// ResolveCommit resolves a revision to the commit it names, directly or
// through tags: `rev-parse --verify --quiet <revision>`, then
// `<id>^{commit}`. Peeling the resolved id keeps revisions such as
// `:/<text>`, whose text runs to the end, intact. The revision must already
// satisfy ValidateRevision.
func (s *Session) ResolveCommit(ctx context.Context, revision string) (string, error) {
	if err := ValidateRevision(revision); err != nil {
		return "", artifact.InternalError("git revision reached resolution unchecked: %v", err)
	}
	id, err := s.runner.revParse(ctx, revision, s.format)
	if err != nil {
		return "", s.runner.classify(ctx, err, revisionNotFound("the revision does not resolve in this repository"))
	}
	commit, err := s.runner.revParse(ctx, id+"^{commit}", s.format)
	if err != nil {
		return "", s.runner.classify(ctx, err, revisionNotFound("the revision names object %s, which is not a commit or does not exist", id))
	}
	return commit, nil
}
