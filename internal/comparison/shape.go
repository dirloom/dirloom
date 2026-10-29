package comparison

import (
	"strconv"

	"github.com/dirloom/dirloom/internal/artifact"
	"github.com/dirloom/dirloom/internal/identity"
)

// shapeID identifies an interned subtree shape. IDs are comparable only
// within one shapeInterner and are never emitted.
type shapeID int

// shapeInterner assigns one ID per distinct shape key. The map compares full
// encoded keys, so two distinct shapes can never share an ID.
type shapeInterner struct {
	ids map[string]shapeID
}

func newShapeInterner() *shapeInterner {
	return &shapeInterner{ids: make(map[string]shapeID)}
}

func (in *shapeInterner) intern(key []byte) shapeID {
	if id, ok := in.ids[string(key)]; ok {
		return id
	}
	id := shapeID(len(in.ids))
	in.ids[string(key)] = id
	return id
}

// treeIndex is the tree view of one side's Identity Projection v1 records
// used by move detection. Node i is records[i].
//
// A shape is a node's kind, target and, recursively, its children's names
// and shapes. The node's own name is excluded, so shapes are rename-invariant.
// The root never moves and has no shape.
type treeIndex struct {
	records  []identity.Record
	byPath   map[artifact.Path]int
	root     int
	parent   []int
	childOff []int
	children []int
	shape    []shapeID
	size     []int
}

// newTreeIndex indexes path-sorted, prefix-closed records and interns every
// non-root shape into interner, which both sides of a comparison share.
func newTreeIndex(records []identity.Record, interner *shapeInterner) (*treeIndex, error) {
	n := len(records)
	idx := &treeIndex{
		records: records,
		byPath:  make(map[artifact.Path]int, n),
		root:    -1,
		parent:  make([]int, n),
		shape:   make([]shapeID, n),
		size:    make([]int, n),
	}
	for i, record := range records {
		if i > 0 && records[i-1].Path >= record.Path {
			return nil, artifact.InternalError("move detection records are not strictly sorted at %q", record.Path)
		}
		idx.byPath[record.Path] = i
		if record.Path == artifact.RootPath {
			idx.root = i
		}
	}
	if idx.root < 0 {
		return nil, artifact.InternalError("move detection records have no root %q", artifact.RootPath)
	}
	// Children in compressed rows: the children of i are
	// children[childOff[i]:childOff[i+1]], in record order, hence by name.
	idx.childOff = make([]int, n+1)
	for i, record := range records {
		if i == idx.root {
			idx.parent[i] = -1
			continue
		}
		parentPath, _ := record.Path.Parent()
		p, ok := idx.byPath[parentPath]
		if !ok {
			return nil, artifact.InternalError("record %q has no parent record", record.Path)
		}
		idx.parent[i] = p
		idx.childOff[p+1]++
	}
	for i := 1; i <= n; i++ {
		idx.childOff[i] += idx.childOff[i-1]
	}
	idx.children = make([]int, idx.childOff[n])
	next := make([]int, n)
	copy(next, idx.childOff[:n])
	for i := range records {
		if p := idx.parent[i]; p >= 0 {
			idx.children[next[p]] = i
			next[p]++
		}
	}
	// A non-root parent sorts before its children ("p" < "p/c"), so a reverse
	// pass sees every child first. The root can sort anywhere ("-a" < ".") and
	// is skipped.
	var key []byte
	for i := n - 1; i >= 0; i-- {
		if i == idx.root {
			continue
		}
		var err error
		key, err = idx.shapeKey(key[:0], i)
		if err != nil {
			return nil, err
		}
		idx.shape[i] = interner.intern(key)
		idx.size[i] = 1
		for _, c := range idx.childrenOf(i) {
			idx.size[i] += idx.size[c]
		}
	}
	return idx, nil
}

// shapeKey appends the unambiguous interning key of node i: its kind code,
// its target when the kind carries one, then each child's name and already
// interned shape ID. Lengths are decimal and ':'-terminated. The key holds
// child IDs, never a child's subtree, so it costs O(children) per node.
func (idx *treeIndex) shapeKey(key []byte, i int) ([]byte, error) {
	record := idx.records[i]
	code, err := record.Kind.Code()
	if err != nil {
		return nil, artifact.InternalError("record %q has unsupported kind %q", record.Path, record.Kind)
	}
	key = append(key, code)
	if record.Kind.HasTarget() {
		key = appendField(key, record.Target)
	}
	children := idx.childrenOf(i)
	key = strconv.AppendInt(key, int64(len(children)), 10)
	key = append(key, ':')
	for _, c := range children {
		key = appendField(key, idx.name(c))
		key = strconv.AppendInt(key, int64(idx.shape[c]), 10)
		key = append(key, ':')
	}
	return key, nil
}

func appendField(key []byte, value string) []byte {
	key = strconv.AppendInt(key, int64(len(value)), 10)
	key = append(key, ':')
	return append(key, value...)
}

func (idx *treeIndex) childrenOf(i int) []int {
	return idx.children[idx.childOff[i]:idx.childOff[i+1]]
}

func (idx *treeIndex) name(i int) string {
	return idx.records[i].Path.Base()
}

func (idx *treeIndex) parentPath(i int) artifact.Path {
	return idx.records[idx.parent[i]].Path
}

func (idx *treeIndex) has(path artifact.Path) bool {
	_, ok := idx.byPath[path]
	return ok
}
