package artifact

// Kind is a closed structural node type. Encoding uses the stable Code()
// values, not Go iota.
type Kind string

const (
	KindDirectory Kind = "directory"
	KindFile      Kind = "file"
	KindSymlink   Kind = "symlink"
	KindJunction  Kind = "junction"
)

const (
	kindCodeDirectory byte = 0x01
	kindCodeFile      byte = 0x02
	kindCodeSymlink   byte = 0x03
	kindCodeJunction  byte = 0x04
)

// Valid reports whether k is one of the v1 kinds.
func (k Kind) Valid() bool {
	switch k {
	case KindDirectory, KindFile, KindSymlink, KindJunction:
		return true
	default:
		return false
	}
}

// Code returns the Canonical Identity Encoding v1 kind byte.
func (k Kind) Code() (byte, error) {
	switch k {
	case KindDirectory:
		return kindCodeDirectory, nil
	case KindFile:
		return kindCodeFile, nil
	case KindSymlink:
		return kindCodeSymlink, nil
	case KindJunction:
		return kindCodeJunction, nil
	default:
		return 0, unsupportedKind(string(k), "")
	}
}

// HasTarget reports whether identity records of this kind include a target.
func (k Kind) HasTarget() bool {
	return k == KindSymlink || k == KindJunction
}

// KindFromCode maps an encoding kind byte back to Kind.
func KindFromCode(code byte) (Kind, error) {
	switch code {
	case kindCodeDirectory:
		return KindDirectory, nil
	case kindCodeFile:
		return KindFile, nil
	case kindCodeSymlink:
		return KindSymlink, nil
	case kindCodeJunction:
		return KindJunction, nil
	default:
		return "", unsupportedKind(sprintfCode(code), "")
	}
}

func sprintfCode(code byte) string {
	const hexdigits = "0123456789abcdef"
	return "0x" + string([]byte{hexdigits[code>>4], hexdigits[code&0x0f]})
}
