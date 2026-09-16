package presentation

import "github.com/dirloom/dirloom/internal/presentation/catalog"

func token(color string, styles []string) Token {
	return Token{Color: color, Styles: append([]string(nil), styles...), Icons: IconPair{}}
}

func binding(color string, styles ...string) Binding {
	return Binding{Color: color, Styles: append([]string(nil), styles...), colorSet: true, stylesSet: len(styles) > 0}
}

func kindBinding(iconColor string) Binding {
	return Binding{IconColor: iconColor, iconColorSet: true}
}

func builtIn(name, description, appearance string, palette map[string]string) Theme {
	roles := make(map[string]Binding, catalog.RoleCount)
	for _, role := range catalog.Roles() {
		styles := []string{}
		switch role {
		case catalog.RoleSecurity:
			styles = []string{"bold"}
		case catalog.RoleGenerated, catalog.RoleVendor:
			styles = []string{"dim"}
		case catalog.RoleContract:
			styles = []string{"bold", "underline"}
		}
		roles[string(role)] = binding(string(role), styles...)
	}
	return Theme{
		SchemaVersion: ThemeFileSchemaVersion, CatalogVersion: catalog.Version,
		Name: name, Description: description, Appearance: appearance,
		Palette: palette,
		Tokens: map[string]Token{
			"tree.edge":      {Color: "edge", Styles: []string{"dim"}},
			"node.directory": token("directory", []string{"bold"}),
			"node.file":      token("file", []string{}),
			"node.symlink":   token("symlink", []string{}),
		},
		Kinds: map[string]Binding{
			"source": kindBinding("source"), "manifest": kindBinding("config"),
			"data": kindBinding("data"), "document": kindBinding("document"),
			"media": kindBinding("media"), "archive": kindBinding("archive"),
			"binary": kindBinding("executable"), "directory": kindBinding("directory"),
			"symlink": kindBinding("symlink"),
		},
		Roles: roles, Rules: []Rule{}, Icons: IconSettings{Spacing: 1},
		Source: Source{Kind: "built-in"}, Warnings: []Warning{},
		Catalog: semanticCatalogSummary(),
	}
}

func defaultPalette() map[string]string {
	return map[string]string{
		"edge": "default", "directory": "ansi:blue", "file": "default", "symlink": "ansi:magenta", "accent": "ansi:cyan",
		"security": "ansi:red", "generated": "ansi:bright-black", "vendor": "ansi:bright-black", "test": "ansi:green",
		"contract": "ansi:yellow", "lock": "ansi:yellow", "infra": "ansi:red", "config": "ansi:yellow",
		"executable": "ansi:green", "archive": "ansi:yellow", "media": "ansi:magenta", "data": "ansi:cyan",
		"source": "ansi:blue", "document": "ansi:magenta", "tooling": "ansi:bright-black", "generic": "default",
	}
}

func midnightPalette() map[string]string {
	return map[string]string{
		"edge": "#9AA5CE", "directory": "#7AA2F7", "file": "#C0CAF5", "symlink": "#BB9AF7", "accent": "#7DCFFF",
		"security": "#FF7C91", "generated": "#9AA5CE", "vendor": "#9AA5CE", "test": "#A8E063",
		"contract": "#FFD166", "lock": "#E8A66A", "infra": "#FF927E", "config": "#F2C879",
		"executable": "#77DDB0", "archive": "#DDB07A", "media": "#FF8FC1", "data": "#7DCFFF",
		"source": "#7AA2F7", "document": "#BB9AF7", "tooling": "#9AA5CE", "generic": "#C0CAF5",
	}
}

func daylightPalette() map[string]string {
	return map[string]string{
		"edge": "#4B5563", "directory": "#1D4ED8", "file": "#111827", "symlink": "#6B21A8", "accent": "#0369A1",
		"security": "#B91C1C", "generated": "#4B5563", "vendor": "#4B5563", "test": "#166534",
		"contract": "#92400E", "lock": "#9A3412", "infra": "#C2410C", "config": "#854D0E",
		"executable": "#047857", "archive": "#92400E", "media": "#9D174D", "data": "#0369A1",
		"source": "#1D4ED8", "document": "#6B21A8", "tooling": "#374151", "generic": "#111827",
	}
}

func vividPalette() map[string]string {
	return map[string]string{
		"edge": "#7A869E", "file": "#F1F5F9", "directory": "#44D7FF", "symlink": "#F38BFF", "accent": "#8B7CFF",
		"security": "#FF5C7C", "generated": "#A1AAC0", "vendor": "#8793AA", "test": "#B6F36B",
		"contract": "#FFE066", "lock": "#FFB86B", "infra": "#FF7A5C", "config": "#FFD166",
		"executable": "#5CFFA9", "archive": "#F4B860", "media": "#FF75D8", "data": "#45E0FF",
		"source": "#66F0C0", "document": "#C9A7FF", "tooling": "#AAB8D0", "generic": "#DEE6F2",
		"icon-directory": "#00D7FF", "icon-symlink": "#FF6BEE", "icon-source": "#00FFD1", "icon-manifest": "#FFB000",
		"icon-data": "#00D4FF", "icon-document": "#A78BFA", "icon-media": "#FF4FB8", "icon-archive": "#FF9F43", "icon-binary": "#2EF2A1",
	}
}

func vividTheme() Theme {
	theme := builtIn(ThemeVivid, "Use a two-tone neon palette designed for high-signal dark terminal output (reference background #10131A).", AppearanceDark, vividPalette())
	theme.Kinds = map[string]Binding{
		"source": kindBinding("icon-source"), "manifest": kindBinding("icon-manifest"),
		"data": kindBinding("icon-data"), "document": kindBinding("icon-document"),
		"media": kindBinding("icon-media"), "archive": kindBinding("icon-archive"),
		"binary": kindBinding("icon-binary"), "directory": kindBinding("icon-directory"),
		"symlink": kindBinding("icon-symlink"),
	}
	theme.Roles[string(catalog.RoleSecurity)] = binding("security", "bold", "underline")
	theme.Roles[string(catalog.RoleTest)] = binding("test", "bold")
	theme.Roles[string(catalog.RoleInfra)] = binding("infra", "bold")
	theme.Roles[string(catalog.RoleExecutable)] = binding("executable", "bold")
	return theme
}

var builtInCatalog = map[string]Theme{
	ThemeDefault:  builtIn(ThemeDefault, "Use the terminal's ANSI palette for universal light and dark background compatibility.", AppearanceUniversal, defaultPalette()),
	ThemeMidnight: builtIn(ThemeMidnight, "Use a truecolor palette designed for dark terminal backgrounds (reference background #1A1B26).", AppearanceDark, midnightPalette()),
	ThemeDaylight: builtIn(ThemeDaylight, "Use a truecolor palette designed for light terminal backgrounds (reference background #FFFFFF).", AppearanceLight, daylightPalette()),
	ThemeVivid:    vividTheme(),
}
