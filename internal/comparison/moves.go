package comparison

import (
	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/identity"
)

// moveKey is a node's name together with its shape.
type moveKey struct {
	name  string
	shape shapeID
}

// occurrence counts how many nodes of one tree share a key or a shape, and
// remembers the last one, which is the only one when count is 1.
type occurrence struct {
	count int
	index int
}

// moveCounts holds the global key and shape counts of one tree: every
// non-root node counts, changed or not.
type moveCounts struct {
	keys   map[moveKey]occurrence
	shapes map[shapeID]occurrence
}

func countMoveKeys(idx *treeIndex) moveCounts {
	counts := moveCounts{
		keys:   make(map[moveKey]occurrence, len(idx.records)),
		shapes: make(map[shapeID]occurrence, len(idx.records)),
	}
	for i := range idx.records {
		if i == idx.root {
			continue
		}
		key := moveKey{name: idx.name(i), shape: idx.shape[i]}
		counts.keys[key] = occurrence{count: counts.keys[key].count + 1, index: i}
		counts.shapes[idx.shape[i]] = occurrence{count: counts.shapes[idx.shape[i]].count + 1, index: i}
	}
	return counts
}

// detectMovesStructuralV1 applies Move Detection structural-v1 to the result
// of the path-anchored merge. It only relabels REMOVED and ADDED records as
// MOVED: CHANGED entries and paths present on both sides are never touched.
// It does no I/O and reads no contents.
func detectMovesStructuralV1(recordsA, recordsB []identity.Record, changes []Change) ([]Change, error) {
	if !hasRemovedAndAdded(changes) {
		return changes, nil
	}
	interner := newShapeInterner()
	treeA, err := newTreeIndex(recordsA, interner)
	if err != nil {
		return nil, err
	}
	treeB, err := newTreeIndex(recordsB, interner)
	if err != nil {
		return nil, err
	}
	countsA := countMoveKeys(treeA)
	countsB := countMoveKeys(treeB)

	absorbedA := make([]bool, len(recordsA))
	absorbedB := make([]bool, len(recordsB))
	// movedFrom[q] is 1 + the A index of the root pair whose destination is
	// q, or 0. Reading it in B order yields moves sorted by destination.
	movedFrom := make([]int, len(recordsB))
	moves := 0
	// A non-root ancestor sorts before its descendants, so the first accepted
	// pair on any branch is its root pair; pairs below it are already
	// absorbed and skipped.
	for p := range recordsA {
		if p == treeA.root || absorbedA[p] || treeB.has(recordsA[p].Path) {
			continue
		}
		q, ok := pairStructuralV1(treeA, treeB, countsA, countsB, p)
		if !ok {
			continue
		}
		if absorbedB[q] {
			return nil, artifact.InternalError("move destination %q is already absorbed", recordsB[q].Path)
		}
		if err := absorb(treeA, treeB, absorbedA, p); err != nil {
			return nil, err
		}
		if err := absorb(treeB, treeA, absorbedB, q); err != nil {
			return nil, err
		}
		movedFrom[q] = p + 1
		moves++
	}
	if moves == 0 {
		return changes, nil
	}

	moved := make([]Change, 0, moves)
	for q, from := range movedFrom {
		if from == 0 {
			continue
		}
		p := from - 1
		if treeA.size[p] != treeB.size[q] {
			return nil, artifact.InternalError("move %q -> %q covers %d and %d nodes", recordsA[p].Path, recordsB[q].Path, treeA.size[p], treeB.size[q])
		}
		evidence := EvidenceName
		if len(treeA.childrenOf(p)) > 0 {
			evidence = EvidenceSubtree
		}
		before := stateOf(recordsA[p])
		after := stateOf(recordsB[q])
		moved = append(moved, Change{
			Path:     recordsB[q].Path,
			Op:       OpMoved,
			From:     recordsA[p].Path,
			Before:   &before,
			After:    &after,
			Evidence: evidence,
			Nodes:    treeA.size[p],
		})
	}

	kept := make([]Change, 0, len(changes))
	for _, change := range changes {
		switch change.Op {
		case OpRemoved:
			if absorbedA[treeA.byPath[change.Path]] {
				continue
			}
		case OpAdded:
			if absorbedB[treeB.byPath[change.Path]] {
				continue
			}
		}
		kept = append(kept, change)
	}
	return mergeChanges(kept, moved), nil
}

// pairStructuralV1 returns the B node paired with the removed A node p, if
// rule R1 (relocation) or R2 (rename in place) accepts one.
func pairStructuralV1(a, b *treeIndex, countsA, countsB moveCounts, p int) (int, bool) {
	key := moveKey{name: a.name(p), shape: a.shape[p]}
	if countsA.keys[key].count == 1 {
		if match := countsB.keys[key]; match.count == 1 && !a.has(b.records[match.index].Path) {
			return match.index, true
		}
	}
	if a.records[p].Kind != artifact.KindDirectory || len(a.childrenOf(p)) == 0 || countsA.shapes[a.shape[p]].count != 1 {
		return 0, false
	}
	match := countsB.shapes[a.shape[p]]
	if match.count != 1 {
		return 0, false
	}
	q := match.index
	if a.has(b.records[q].Path) || b.parentPath(q) != a.parentPath(p) || b.name(q) == a.name(p) {
		return 0, false
	}
	return q, true
}

// absorb marks the subtree rooted at i. Every absorbed node must be absent
// from the other side: records are prefix-closed, so a removed (or added)
// node's whole subtree is removed (or added).
func absorb(idx, other *treeIndex, absorbed []bool, i int) error {
	stack := []int{i}
	for len(stack) > 0 {
		n := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if absorbed[n] || other.has(idx.records[n].Path) {
			return artifact.InternalError("cannot absorb %q into a move", idx.records[n].Path)
		}
		absorbed[n] = true
		stack = append(stack, idx.childrenOf(n)...)
	}
	return nil
}

func hasRemovedAndAdded(changes []Change) bool {
	removed, added := false, false
	for _, change := range changes {
		switch change.Op {
		case OpRemoved:
			removed = true
		case OpAdded:
			added = true
		}
		if removed && added {
			return true
		}
	}
	return false
}

// mergeChanges merges two path-sorted change lists in one pass.
func mergeChanges(a, b []Change) []Change {
	out := make([]Change, 0, len(a)+len(b))
	i, j := 0, 0
	for i < len(a) && j < len(b) {
		if a[i].Path < b[j].Path {
			out = append(out, a[i])
			i++
		} else {
			out = append(out, b[j])
			j++
		}
	}
	out = append(out, a[i:]...)
	return append(out, b[j:]...)
}
