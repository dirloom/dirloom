package filter

// DefaultDirectories is deliberately short and conservative. These names are
// matched exactly and only when the entry is a directory.
var DefaultDirectories = map[string]struct{}{
	".git":         {},
	"node_modules": {},
	".next":        {},
	".nuxt":        {},
	"coverage":     {},
	".cache":       {},
	".turbo":       {},
}
