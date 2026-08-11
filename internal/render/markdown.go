package render

import (
	"fmt"
	"io"

	"github.com/dirloom/dirloom/internal/tree"
)

type markdownRenderer struct {
	text textRenderer
}

func (r markdownRenderer) Render(w io.Writer, root *tree.Node) error {
	if _, err := fmt.Fprintln(w, "```text"); err != nil {
		return err
	}
	if err := r.text.Render(w, root); err != nil {
		return err
	}
	_, err := fmt.Fprintln(w, "```")
	return err
}
