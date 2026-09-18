package cli

import "strings"

type helpTopic struct {
	Name    string
	Aliases []string
	Summary string
	Body    string
	SeeAlso []string
}

// helpTopics is the compiled, ordered catalog of conceptual CLI topics.
// Order is alphabetical by Name and is the display order for `dirloom help topics`.
var helpTopics = []helpTopic{
	topicEntry("colors", "Terminal color behavior", strings.TrimSpace(`
COLOR MODES

Controls whether Dirloom emits ANSI color in terminal text output.

Usage:
  dirloom [command] --color[=MODE]

Modes:
  auto      Emit ANSI only on an eligible interactive TTY (default)
  always    Force ANSI, including pipes and --output
  never     Never emit ANSI

Environment:
  NO_COLOR  A non-empty value disables automatic and configured color.
            Only an explicit CLI --color=always overrides it.
  TTY       Automatic color requires an interactive terminal, no --output,
            an empty CI variable, and TERM other than dumb.

Notes:
  --color without a value is equivalent to --color=auto.
  Help, diagnostics, and errors stay uncolored.
  Canonical and machine-oriented formats stay uncolored.

Examples:
  dirloom --color
  dirloom --color=never
  dirloom --color=always --output tree.txt
`)+"\n", "themes", "icons", "formats"),
	topicEntry("concepts", "Index of conceptual help topics", strings.TrimSpace(`
CONCEPT INDEX

A logical grouping of compiled help topics. The exhaustive list remains
available through dirloom help topics.

Presentation
  icons
  colors
  themes

Structure
  filters
  formats

Configuration
  configuration
  presets

Output
  output
  diagrams

Workflows
  examples

Run 'dirloom help <topic>' for details.
Run 'dirloom help topics' for the alphabetical catalog.
`)+"\n"),
	topicEntry("configuration", "Configuration resolution and precedence", strings.TrimSpace(`
CONFIGURATION

Dirloom resolves inspect options from layered sources. All sources are local
files or CLI flags; nothing is downloaded.

Precedence (highest first):
  CLI > project > user > built-in

Sources:
  CLI       Explicit flags such as --depth, --format, and --preset
  project   .dirloom.yaml discovered from the inspected root
  user      The native user configuration file
  built-in  Compiled defaults (including icons: never and color: auto)

Useful commands:
  dirloom --config FILE     Use an explicit project file
  dirloom --no-user-config  Ignore personal configuration
  dirloom --no-config       Ignore user and project files

help vs explain:
  dirloom help configuration   Teaches the model and precedence
  dirloom config explain       Reports the values actually resolved

See the persistent configuration guide for the schema and discovery rules.

Notes:
  terminal.capabilities.nerdFont is USER CONFIG ONLY.
  DIRLOOM_NERD_FONT is a runtime declaration used by --icons auto.
`)+"\n", "presets", "themes", "filters", "icons"),
	topicEntry("diagrams", "Mermaid, Graphviz and D2 exports", strings.TrimSpace(`
DIAGRAMS

Dirloom emits diagram source, not rendered PNG or SVG images. Render the
source with Mermaid CLI, Graphviz, D2, or any compatible viewer.

Formats:
  mermaid    Mermaid flowchart source
  graphviz   Graphviz DOT source (alias: dot)
  d2         D2 source

Options (diagram formats only):
  --diagram-view structure              Projection (currently structure)
  --diagram-direction top-down|left-right
  --diagram-max-nodes N|unlimited       Optional node budget

Examples:
  dirloom --format mermaid
  dirloom --format graphviz --diagram-direction left-right
  dirloom --format d2 --output structure.d2

Notes:
  Diagram flags are rejected for text, Markdown, and JSON.
  Large graphs print a warning unless --diagram-max-nodes is set.
  Canonical diagram bytes stay undecorated.
`)+"\n", "formats", "filters", "output"),
	topicEntry("examples", "Common command recipes", strings.TrimSpace(`
EXAMPLES

Inspect current project
  dirloom

Compact overview
  dirloom --preset compact

Pretty interactive terminal view
  dirloom --theme vivid --icons

Markdown for documentation
  dirloom --format markdown

Semantic Markdown tree
  dirloom --format markdown-tree

Machine contract
  dirloom --format json

Mermaid source
  dirloom --format mermaid

Copy Markdown to the clipboard
  dirloom --format markdown --copy

Explain resolved configuration
  dirloom config explain
`)+"\n", "icons", "formats", "output", "presets"),
	topicEntry("filters", "Depth, ignore rules and visibility", strings.TrimSpace(`
FILTERS

Controls which entries appear in the inspected tree. The explicit root is
always retained.

Flags:
  --depth N|unlimited   Maximum depth (0 prints only the root)
  --dirs-only           Include directories only
  --hidden              Include hidden entries that survive other filters
  --ignore PATTERN      Exclude a pattern (repeatable)
  --no-default-ignore   Disable built-in directory exclusions
  --no-gitignore        Do not apply .gitignore files

Evaluation order (first exclusion wins):
  1. the --output destination
  2. built-in directory exclusions
  3. preset and explicit ignore rules (user, project, then CLI)
  4. scoped .gitignore rules
  5. hidden-entry visibility

Examples:
  dirloom --depth 3
  dirloom --dirs-only --preset compact
  dirloom --ignore node_modules --ignore dist
`)+"\n", "presets", "formats", "output"),
	topicEntry("formats", "Text, Markdown, JSON and diagram formats", strings.TrimSpace(`
FORMATS

Selects the public output contract.

Human presentation
  text            Unicode or ASCII tree (default)
  markdown        Fenced text tree for documents

Canonical / machine output
  markdown-tree   Nested Markdown list
  json            Versioned machine contract

Diagram source
  mermaid         Mermaid flowchart
  graphviz        Graphviz DOT (alias: dot)
  d2              D2

Usage:
  dirloom --format NAME

Notes:
  text may use --style unicode|ascii and terminal presentation.
  markdown may use --style; it stays uncolored and without presentation icons.
  json, markdown-tree, and diagram formats stay canonical: no ANSI, no icons.
  --style is rejected with json, markdown-tree, and diagram formats.

Examples:
  dirloom --format markdown --copy
  dirloom --format json --output structure.json
  dirloom --format mermaid
`)+"\n", "diagrams", "output", "icons", "colors"),
	topicEntry("icons", "ASCII, Unicode, Nerd Font and automatic icons", strings.TrimSpace(`
ICON MODES

Controls how Dirloom renders icons in terminal output.

Usage:
  dirloom [command] --icons[=MODE]

Modes:
  auto       Use Unicode, or Nerd when a Nerd Font capability is declared
  ascii      Use a strict printable-ASCII catalog
  unicode    Use portable Unicode icons
  nerd       Use Nerd Font glyphs
  never      Disable icons (built-in default when the flag is omitted)

Examples:
  dirloom --icons
  dirloom --icons=ascii
  dirloom --icons=unicode
  dirloom --theme vivid --icons
  dirloom --theme midnight --icons=nerd

Notes:
  --icons without a value is equivalent to --icons=auto.
  Auto does not detect fonts and does not infer Nerd support from the terminal.
  Auto uses Nerd only when DIRLOOM_NERD_FONT or user config declares it.
  Otherwise auto selects portable Unicode.
  Selecting a theme does not enable icons by itself.
  Canonical and machine-oriented outputs remain undecorated.
`)+"\n", "themes", "colors", "formats"),
	topicEntry("output", "stdout, clipboard and transactional files", strings.TrimSpace(`
OUTPUT

Dirloom writes one destination per invocation.

Destinations:
  stdout     Default. Keep it clean: diagnostics go to stderr.
  --output   Transactional file write; stdout stays empty on success.
  --copy     Native clipboard instead of stdout; exclusive with --output.
  pipe       Shell redirection of stdout (for example > tree.txt)

Guarantees:
  UTF-8 without BOM, LF line endings, exactly one trailing newline.
  --output replaces the file atomically and does not leave a partial tree.
  A clipboard or write failure does not reprint the tree on stdout.
  Help, errors, and machine formats stay canonical.

Examples:
  dirloom --format markdown --output structure.md
  dirloom --format markdown --copy
  dirloom --format json > structure.json
`)+"\n", "formats", "diagrams", "filters"),
	topicEntry("presets", "Built-in project-tree presets", strings.TrimSpace(`
PRESETS

Built-in, inspectable starting points for common inspect workflows.

Presets:
  ai        Markdown source structure for AI workflows
  compact   Shallow directory-only overview
  docs      Markdown for documentation and reviews
  monorepo  Workspace topology without dist/build noise
  none      Neutralize a preset inherited from configuration

Usage:
  dirloom --preset NAME

help vs explain:
  dirloom help presets         Teaches the preset concept
  dirloom preset explain NAME  Inspects one built-in definition

Notes:
  Explicit CLI flags override individual preset values.
  Presets do not change canonical format contracts.
`)+"\n", "configuration", "filters", "formats"),
	topicEntry("themes", "Built-in and custom terminal themes", strings.TrimSpace(`
THEMES

A theme binds colors and glyphs to semantic kinds. It is independent from
whether color or icons are enabled.

Do not confuse:
  theme   Which palette and glyph bindings to use
  color   Whether ANSI color is emitted (never, always, auto)
  icons   Whether glyphs are emitted (never, ascii, unicode, nerd, auto)

Built-in themes:
  default    Terminal ANSI palette
  midnight   Dark
  daylight   Light
  vivid      Dark two-tone neon

Examples:
  dirloom --theme vivid
  dirloom --theme vivid --icons
  dirloom --theme midnight --icons nerd
  dirloom theme list
  dirloom theme explain vivid

help vs explain:
  dirloom help themes          Teaches the theme system
  dirloom theme explain NAME   Inspects one built-in or YAML theme

Notes:
  A theme alone does not turn icons on; pass --icons or --icons=MODE.
  Custom themes are local YAML files. Theme flags are rejected for
  canonical machine and diagram formats.
`)+"\n", "icons", "colors", "formats"),
}

func topicEntry(name, summary, body string, seeAlso ...string) helpTopic {
	return helpTopic{
		Name:    name,
		Summary: summary,
		Body:    body,
		SeeAlso: append([]string(nil), seeAlso...),
	}
}

func lookupHelpTopic(name string) (helpTopic, bool) {
	for _, topic := range helpTopics {
		if topic.Name == name {
			return topic, true
		}
		for _, alias := range topic.Aliases {
			if alias == name {
				return topic, true
			}
		}
	}
	return helpTopic{}, false
}

func helpTopicNames() []string {
	names := make([]string, 0, len(helpTopics))
	for _, topic := range helpTopics {
		names = append(names, topic.Name)
	}
	return names
}

func helpTopicRegistryProblems() []string {
	var problems []string
	names := make(map[string]string, len(helpTopics))
	aliases := make(map[string]string)
	previous := ""
	for _, topic := range helpTopics {
		if topic.Name == "" {
			problems = append(problems, "empty topic name")
			continue
		}
		if topic.Summary == "" {
			problems = append(problems, "empty summary for "+topic.Name)
		}
		if strings.TrimSpace(topic.Body) == "" {
			problems = append(problems, "empty body for "+topic.Name)
		}
		if existing, ok := names[topic.Name]; ok {
			problems = append(problems, "duplicate topic name "+topic.Name+" (also "+existing+")")
		}
		if existing, ok := aliases[topic.Name]; ok {
			problems = append(problems, "topic name "+topic.Name+" collides with alias of "+existing)
		}
		names[topic.Name] = topic.Name
		if previous != "" && topic.Name < previous {
			problems = append(problems, "topics are not in alphabetical order: "+topic.Name+" follows "+previous)
		}
		previous = topic.Name
		for _, alias := range topic.Aliases {
			if alias == "" {
				problems = append(problems, "empty alias for "+topic.Name)
				continue
			}
			if existing, ok := names[alias]; ok {
				problems = append(problems, "alias "+alias+" of "+topic.Name+" collides with topic "+existing)
			}
			if existing, ok := aliases[alias]; ok {
				problems = append(problems, "duplicate alias "+alias+" ("+existing+" and "+topic.Name+")")
			}
			aliases[alias] = topic.Name
		}
	}
	for _, topic := range helpTopics {
		seen := make(map[string]struct{}, len(topic.SeeAlso))
		for _, related := range topic.SeeAlso {
			if related == "" {
				problems = append(problems, topic.Name+" has an empty SeeAlso entry")
				continue
			}
			if related == topic.Name {
				problems = append(problems, topic.Name+" SeeAlso references itself")
				continue
			}
			if _, ok := seen[related]; ok {
				problems = append(problems, topic.Name+" has duplicate SeeAlso "+related)
				continue
			}
			seen[related] = struct{}{}
			if _, ok := lookupHelpTopic(related); !ok {
				problems = append(problems, topic.Name+" SeeAlso references unknown topic "+related)
			}
		}
	}
	return problems
}
