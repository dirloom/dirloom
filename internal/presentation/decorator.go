package presentation

import (
	"strings"

	"github.com/dirloom/dirloom/internal/render"
)

// Decorator implements render.Decorator for a resolved terminal capability set.
type Decorator struct {
	theme   *CompiledTheme
	color   bool
	icons   string
	profile ColorProfile
}

// NewDecorator creates a terminal projection. Disabled capabilities are neutral.
func NewDecorator(theme *CompiledTheme, colorEnabled bool, iconMode string, profile ColorProfile) *Decorator {
	return &Decorator{theme: theme, color: colorEnabled, icons: iconMode, profile: profile}
}

// Edge styles one complete connector segment and closes every ANSI sequence.
func (decorator *Decorator) Edge(value string) string {
	if decorator == nil || decorator.theme == nil || !decorator.color {
		return value
	}
	token := decorator.theme.edge()
	return ansiStyle(value, token.color, token.styles, decorator.profile)
}

// Node decorates a displayed name without changing node identity or ordering.
func (decorator *Decorator) Node(context render.NodeContext) string {
	if decorator == nil || decorator.theme == nil {
		return context.Display
	}
	display := EscapeTerminalText(context.Display)
	style := decorator.theme.resolve(context.Path, context.Name, context.Type)
	icon := ""
	switch decorator.icons {
	case IconsNerd:
		icon = style.icons.Nerd
		if icon == "" {
			icon = style.icons.Unicode
		}
	case IconsUnicode:
		icon = style.icons.Unicode
	}
	if icon != "" {
		display = icon + strings.Repeat(" ", decorator.theme.iconSpacing) + display
	}
	if decorator.color {
		display = ansiStyle(display, style.color, style.styles, decorator.profile)
	}
	return display
}
