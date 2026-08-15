package presentation

var baseIconRules = []Rule{
	{Match: Match{Name: "README.md"}, Color: "accent", Styles: []string{"bold"}, Icons: IconPair{Unicode: "¶", Nerd: "󰍔"}},
	{Match: Match{Extension: ".md"}, Color: "accent", Icons: IconPair{Unicode: "¶", Nerd: "󰍔"}},
	{Match: Match{Extension: ".go"}, Icons: IconPair{Unicode: "•", Nerd: "󰟓"}},
	{Match: Match{Extension: ".ts"}, Icons: IconPair{Unicode: "•", Nerd: "󰛦"}},
	{Match: Match{Extension: ".tsx"}, Icons: IconPair{Unicode: "•", Nerd: "󰛦"}},
	{Match: Match{Extension: ".js"}, Icons: IconPair{Unicode: "•", Nerd: "󰌞"}},
	{Match: Match{Extension: ".jsx"}, Icons: IconPair{Unicode: "•", Nerd: "󰌞"}},
	{Match: Match{Extension: ".json"}, Icons: IconPair{Unicode: "◇", Nerd: "󰘦"}},
	{Match: Match{Extension: ".yaml"}, Icons: IconPair{Unicode: "◇", Nerd: "󰈙"}},
	{Match: Match{Extension: ".yml"}, Icons: IconPair{Unicode: "◇", Nerd: "󰈙"}},
	{Match: Match{Extension: ".toml"}, Icons: IconPair{Unicode: "◇", Nerd: "󰈙"}},
	{Match: Match{Name: "Dockerfile"}, Icons: IconPair{Unicode: "▣", Nerd: "󰡨"}},
}

func builtIn(name, description, appearance string, palette map[string]string) Theme {
	return Theme{
		SchemaVersion: SchemaVersion,
		Name:          name,
		Description:   description,
		Appearance:    appearance,
		Palette:       palette,
		Tokens: map[string]Token{
			"tree.edge":      {Color: "edge", Styles: []string{"dim"}},
			"node.directory": {Color: "directory", Styles: []string{"bold"}, Icons: IconPair{Unicode: "▸", Nerd: "󰉋"}},
			"node.file":      {Color: "file", Styles: []string{}, Icons: IconPair{Unicode: "·", Nerd: "󰈔"}},
			"node.symlink":   {Color: "symlink", Styles: []string{}, Icons: IconPair{Unicode: "↗", Nerd: "󰌷"}},
		},
		Rules:    cloneRules(baseIconRules),
		Icons:    IconSettings{Spacing: 1},
		Source:   Source{Kind: "built-in"},
		Warnings: []Warning{},
	}
}

var builtInCatalog = map[string]Theme{
	ThemeDefault: builtIn(
		ThemeDefault,
		"Use the terminal's ANSI palette for universal light and dark background compatibility.",
		AppearanceUniversal,
		map[string]string{
			"edge": "default", "directory": "ansi:blue", "file": "default", "symlink": "ansi:magenta", "accent": "ansi:cyan",
		},
	),
	ThemeMidnight: builtIn(
		ThemeMidnight,
		"Use a truecolor palette designed for dark terminal backgrounds (reference background #1A1B26).",
		AppearanceDark,
		map[string]string{
			"edge": "#9AA5CE", "directory": "#7AA2F7", "file": "#C0CAF5", "symlink": "#BB9AF7", "accent": "#7DCFFF",
		},
	),
	ThemeDaylight: builtIn(
		ThemeDaylight,
		"Use a truecolor palette designed for light terminal backgrounds (reference background #FFFFFF).",
		AppearanceLight,
		map[string]string{
			"edge": "#4B5563", "directory": "#1D4ED8", "file": "#111827", "symlink": "#6B21A8", "accent": "#0369A1",
		},
	),
}
