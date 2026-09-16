package presentation

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/dirloom/dirloom/internal/presentation/catalog"
	"github.com/dirloom/dirloom/internal/tree"
)

func TestPublicThemeV1RejectsPrototypeAndSeparatesContractVersions(t *testing.T) {
	if ThemeFileSchemaVersion != 1 || ThemeListSchemaVersion != 1 || ThemeExplainSchemaVersion != 1 || ThemeValidateSchemaVersion != 1 || ThemeClassifySchemaVersion != 1 {
		t.Fatal("public theme and diagnostic contract versions must remain explicitly versioned")
	}
	prototype := []byte("schemaVersion: 1\nname: prototype\nappearance: dark\ntokens: {}\n")
	_, err := parseTheme(prototype, "prototype.yaml")
	if err == nil || !IsInvalid(err) || !strings.Contains(err.Error(), "catalogVersion is required for theme schemaVersion 1") {
		t.Fatalf("prototype error = %v", err)
	}
	for _, document := range []string{
		"catalogVersion: 1\nname: missing-schema\nappearance: dark\n",
		"schemaVersion: 2\ncatalogVersion: 1\nname: future\nappearance: dark\n",
		"schemaVersion: 1\ncatalogVersion: 2\nname: future\nappearance: dark\n",
	} {
		if _, parseErr := parseTheme([]byte(document), "invalid.yaml"); parseErr == nil || !IsInvalid(parseErr) {
			t.Fatalf("document unexpectedly accepted: %q", document)
		}
	}
}

func TestThemeV1KindRoleBindingsNullResetsAndResolution(t *testing.T) {
	theme, err := parseTheme([]byte(`schemaVersion: 1
catalogVersion: 1
name: semantic
appearance: dark
palette:
  customText: "#E5E9F0"
  customIcon: "#65D6BA"
tokens:
  node.file:
    iconColor: customIcon
kinds:
  source:
    iconColor: customIcon
  source.go:
    icons:
      unicode: "G"
      nerd: null
roles:
  test:
    color: customText
    styles: []
rules:
  - match: {path: tools/codegen.go}
    kind: document.markdown
    role: contract
    color: customText
    iconColor: null
    styles: []
    icons:
      unicode: null
      nerd: null
`), "semantic.yaml")
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := Compile(theme)
	if err != nil {
		t.Fatal(err)
	}

	testFile := compiled.Inspect("internal/api/user_test.go", "user_test.go", tree.NodeFile)
	if testFile.Classification.Kind != "source.go" || !reflect.DeepEqual(testFile.Classification.Roles, []catalog.Role{catalog.RoleTest, catalog.RoleSource}) || testFile.VisualRole != catalog.RoleTest {
		t.Fatalf("test classification = %#v", testFile)
	}
	if testFile.Icons.Unicode != "G" || testFile.Icons.Nerd != "" || testFile.TextColor != "#E5E9F0" || testFile.IconColor != "#65D6BA" || len(testFile.Styles) != 0 {
		t.Fatalf("test style = %#v", testFile)
	}

	generated := compiled.Inspect("proto/user.pb.go", "user.pb.go", tree.NodeFile)
	if generated.Classification.Kind != "source.go" || generated.Classification.Roles[0] != catalog.RoleGenerated || generated.VisualRole != catalog.RoleGenerated || generated.Icons.Unicode != "G" {
		t.Fatalf("generated style = %#v", generated)
	}

	overridden := compiled.Inspect("tools/codegen.go", "codegen.go", tree.NodeFile)
	if overridden.Classification.Kind != "document.markdown" || overridden.VisualRole != catalog.RoleContract {
		t.Fatalf("rule classification = %#v", overridden)
	}
	if overridden.Icons.Unicode != "" || overridden.Icons.Nerd != "" || overridden.TextColor != "#E5E9F0" || overridden.IconColor != "#E5E9F0" || len(overridden.Styles) != 0 {
		t.Fatalf("rule reset style = %#v", overridden)
	}
	if overridden.Origins.Kind != "theme-rule" || overridden.Origins.Role != "theme-rule" || overridden.Origins.TextColor != "theme-rule" || overridden.Origins.IconColor != "theme-rule" || overridden.Origins.Icons != "theme-rule" {
		t.Fatalf("origins = %#v", overridden.Origins)
	}
}

func TestThemeV1NullIconColorFollowsLaterTextOverrides(t *testing.T) {
	theme, err := parseTheme([]byte(`schemaVersion: 1
catalogVersion: 1
name: follow
appearance: dark
palette:
  text: "#E5E9F0"
kinds:
  source:
    iconColor: null
roles:
  source:
    color: text
`), "follow.yaml")
	if err != nil {
		t.Fatal(err)
	}
	compiled, err := Compile(theme)
	if err != nil {
		t.Fatal(err)
	}

	inspection := compiled.Inspect("main.go", "main.go", tree.NodeFile)
	if inspection.TextColor != "#E5E9F0" || inspection.IconColor != "#E5E9F0" {
		t.Fatalf("icon color did not follow the later text override: %#v", inspection)
	}
	if inspection.Origins.TextColor != "theme-role" || inspection.Origins.IconColor != "theme-role" {
		t.Fatalf("followed color origins = %#v", inspection.Origins)
	}
}
func TestThemeV1UnknownBindingsWarnButUnknownRuleActionsFail(t *testing.T) {
	theme, err := parseTheme([]byte(`schemaVersion: 1
catalogVersion: 1
name: future
appearance: universal
tokens:
  node.future: {color: default}
kinds:
  source.future: {color: default}
roles:
  state.future: {color: default}
`), "future.yaml")
	if err != nil {
		t.Fatal(err)
	}
	wantCodes := []string{"unknown-token", "unknown-kind-binding", "unknown-role-binding"}
	gotCodes := make([]string, len(theme.Warnings))
	for index, warning := range theme.Warnings {
		gotCodes[index] = warning.Code
	}
	if !reflect.DeepEqual(gotCodes, wantCodes) {
		t.Fatalf("warning codes = %#v, want %#v", gotCodes, wantCodes)
	}
	for name, action := range map[string]string{
		"kind":      "kind: source.future",
		"role":      "role: state.future",
		"no-action": "",
	} {
		document := fmt.Sprintf("schemaVersion: 1\ncatalogVersion: 1\nname: invalid\nappearance: universal\nrules:\n  - match: {name: a}\n    %s\n", action)
		if _, parseErr := parseTheme([]byte(document), name+".yaml"); parseErr == nil || !IsInvalid(parseErr) {
			t.Fatalf("%s unexpectedly accepted: %v", name, parseErr)
		}
	}
}

func TestThemeV1CollectionLimits(t *testing.T) {
	tests := []struct {
		name   string
		prefix string
		count  int
		entry  func(int) string
	}{
		{"palette", "palette:\n", maxPalette + 1, func(index int) string { return fmt.Sprintf("  c%d: default\n", index) }},
		{"kinds", "kinds:\n", maxKindBindings + 1, func(index int) string { return fmt.Sprintf("  future.%d: {color: default}\n", index) }},
		{"roles", "roles:\n", maxRoleBindings + 1, func(index int) string { return fmt.Sprintf("  future%d: {color: default}\n", index) }},
		{"rules", "rules:\n", maxRules + 1, func(index int) string { return fmt.Sprintf("  - match: {name: file%d}\n    color: default\n", index) }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			var builder strings.Builder
			builder.WriteString("schemaVersion: 1\ncatalogVersion: 1\nname: limits\nappearance: universal\n")
			builder.WriteString(test.prefix)
			for index := 0; index < test.count; index++ {
				builder.WriteString(test.entry(index))
			}
			if _, err := parseTheme([]byte(builder.String()), test.name+".yaml"); err == nil || !IsInvalid(err) || !strings.Contains(err.Error(), "limit") {
				t.Fatalf("limit error = %v", err)
			}
		})
	}
}
