package config

import (
	"bytes"
	"path/filepath"
	"strings"
	"testing"
)

func TestUserNerdFontCapabilityAndProjectRejection(t *testing.T) {
	root := t.TempDir()
	userBase := t.TempDir()
	writeConfig(t, filepath.Join(userBase, "dirloom", "config.yaml"), `schemaVersion: 1
terminal:
  capabilities:
    nerdFont: true
`)
	loader := NewLoader(WithUserConfigDir(func() (string, error) { return userBase, nil }))
	resolved, err := loader.Resolve(ResolveOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.Effective.NerdFont.Set || !resolved.Effective.NerdFont.Value {
		t.Fatalf("user nerdFont = %#v", resolved.Effective.NerdFont)
	}
	if resolved.Provenance["terminal.capabilities.nerdFont"].Source != SourceUser {
		t.Fatalf("nerdFont provenance = %#v", resolved.Provenance["terminal.capabilities.nerdFont"])
	}
	if pointer := resolved.ConfiguredNerdFont(); pointer == nil || !*pointer {
		t.Fatalf("configured pointer = %v", pointer)
	}

	var text bytes.Buffer
	if err := resolved.WriteText(&text); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(text.String(), "capabilities.nerdFont: true (user:") {
		t.Fatalf("text diagnostic = %q", text.String())
	}

	writeConfig(t, filepath.Join(userBase, "dirloom", "config.yaml"), `schemaVersion: 1
terminal:
  capabilities:
    nerdFont: false
`)
	resolved, err = loader.Resolve(ResolveOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if !resolved.Effective.NerdFont.Set || resolved.Effective.NerdFont.Value {
		t.Fatalf("user nerdFont false = %#v", resolved.Effective.NerdFont)
	}

	absent, err := loaderWithoutUserConfig().Resolve(ResolveOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if absent.Effective.NerdFont.Set || absent.ConfiguredNerdFont() != nil {
		t.Fatalf("absent capability = %#v", absent.Effective.NerdFont)
	}

	writeConfig(t, filepath.Join(root, ".dirloom.yaml"), `schemaVersion: 1
terminal:
  capabilities:
    nerdFont: true
`)
	if _, err := loaderWithoutUserConfig().Resolve(ResolveOptions{Root: root}); err == nil || !IsInvalid(err) || !strings.Contains(err.Error(), "cannot be declared by project configuration") {
		t.Fatalf("project true = %v", err)
	}
	writeConfig(t, filepath.Join(root, ".dirloom.yaml"), `schemaVersion: 1
terminal:
  capabilities:
    nerdFont: false
`)
	if _, err := loaderWithoutUserConfig().Resolve(ResolveOptions{Root: root}); err == nil || !IsInvalid(err) || !strings.Contains(err.Error(), "cannot be declared by project configuration") {
		t.Fatalf("project false = %v", err)
	}

	explicit := filepath.Join(t.TempDir(), "team.yaml")
	writeConfig(t, explicit, `schemaVersion: 1
terminal:
  capabilities:
    nerdFont: true
`)
	if _, err := loaderWithoutUserConfig().Resolve(ResolveOptions{Root: root, ExplicitProjectPath: explicit}); err == nil || !IsInvalid(err) {
		t.Fatalf("--config capability = %v", err)
	}

	disabledUser, err := loader.Resolve(ResolveOptions{Root: t.TempDir(), DisableUser: true})
	if err != nil {
		t.Fatal(err)
	}
	if disabledUser.Effective.NerdFont.Set {
		t.Fatal("--no-user-config must drop the user capability")
	}
	disabledAll, err := loader.Resolve(ResolveOptions{Root: t.TempDir(), DisableAll: true})
	if err != nil {
		t.Fatal(err)
	}
	if disabledAll.Effective.NerdFont.Set {
		t.Fatal("--no-config must drop the user capability")
	}
}

func TestNerdFontYAMLMustBeBoolean(t *testing.T) {
	if _, err := parseDocument([]byte("schemaVersion: 1\nterminal:\n  capabilities:\n    nerdFont: maybe\n"), "config.yaml"); err == nil || !strings.Contains(err.Error(), "terminal.capabilities.nerdFont must be a boolean") {
		t.Fatalf("invalid nerdFont = %v", err)
	}
	if _, err := parseDocument([]byte("schemaVersion: 1\nterminal:\n  future: true\n"), "config.yaml"); err == nil || !strings.Contains(err.Error(), "field future not found") {
		t.Fatalf("unknown terminal field = %v", err)
	}
	values, err := parseDocument([]byte("schemaVersion: 1\nterminal:\n  capabilities:\n    nerdFont: true\n"), "config.yaml")
	if err != nil || !values.NerdFont.Set || !values.NerdFont.Value {
		t.Fatalf("parsed nerdFont = %#v err=%v", values.NerdFont, err)
	}
}

func TestResolvePresentationAcceptsASCIIIcons(t *testing.T) {
	root := t.TempDir()
	writeConfig(t, filepath.Join(root, ".dirloom.yaml"), "schemaVersion: 1\npresentation:\n  icons: ascii\n")
	resolved, err := loaderWithoutUserConfig().Resolve(ResolveOptions{Root: root})
	if err != nil {
		t.Fatal(err)
	}
	if resolved.Effective.Icons != "ascii" {
		t.Fatalf("icons = %q", resolved.Effective.Icons)
	}
}
