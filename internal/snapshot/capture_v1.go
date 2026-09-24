package snapshot

// CaptureV1 records effective structural observation rules for Snapshot v1.
// Depth nil means unlimited. Ignore preserves effective pattern order.
type CaptureV1 struct {
	Depth             *int     `json:"depth"`
	DirsOnly          bool     `json:"dirsOnly"`
	Hidden            bool     `json:"hidden"`
	UseDefaultIgnores bool     `json:"useDefaultIgnores"`
	UseGitignore      bool     `json:"useGitignore"`
	Ignore            []string `json:"ignore"`
}

// Clone returns a deep copy of capture semantics.
func (c CaptureV1) Clone() CaptureV1 {
	out := c
	if c.Depth != nil {
		value := *c.Depth
		out.Depth = &value
	}
	out.Ignore = append([]string(nil), c.Ignore...)
	if out.Ignore == nil {
		out.Ignore = []string{}
	}
	return out
}

// NormalizeIgnore ensures Ignore is a non-nil empty slice when absent.
func (c *CaptureV1) NormalizeIgnore() {
	if c.Ignore == nil {
		c.Ignore = []string{}
	}
}
