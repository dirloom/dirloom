package presentation

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"testing"
)

func TestCapabilityResolution(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		tty     bool
		request CapabilityRequest
		color   bool
		icons   string
		profile ColorProfile
	}{
		{"auto tty", nil, true, CapabilityRequest{Format: "text", ColorMode: "auto", IconMode: "auto"}, true, "unicode", ProfileANSI16},
		{"auto pipe", nil, false, CapabilityRequest{Format: "text", ColorMode: "auto", IconMode: "auto"}, false, "unicode", ProfileTrueColor},
		{"ci", map[string]string{"CI": "1"}, true, CapabilityRequest{Format: "text", ColorMode: "auto", IconMode: "auto"}, false, "unicode", ProfileANSI16},
		{"dumb", map[string]string{"TERM": "dumb"}, true, CapabilityRequest{Format: "text", ColorMode: "auto", IconMode: "auto"}, false, "unicode", ProfileANSI16},
		{"output", nil, true, CapabilityRequest{Format: "text", ColorMode: "auto", IconMode: "auto", OutputPath: "tree.txt"}, false, "unicode", ProfileTrueColor},
		{"forced pipe", nil, false, CapabilityRequest{Format: "text", ColorMode: "always", IconMode: "nerd"}, true, "nerd", ProfileTrueColor},
		{"no color", map[string]string{"NO_COLOR": "1"}, true, CapabilityRequest{Format: "text", ColorMode: "always", IconMode: "unicode"}, false, "unicode", ProfileANSI16},
		{"CLI overrides no color", map[string]string{"NO_COLOR": "1", "COLORTERM": "truecolor"}, true, CapabilityRequest{Format: "text", ColorMode: "always", IconMode: "unicode", ColorExplicitCLI: true}, true, "unicode", ProfileTrueColor},
		{"256", map[string]string{"TERM": "xterm-256color"}, true, CapabilityRequest{Format: "text", ColorMode: "auto", IconMode: "auto"}, true, "unicode", ProfileANSI256},
		{"machine", nil, true, CapabilityRequest{Format: "json", ColorMode: "always", IconMode: "nerd"}, false, "never", ProfileTrueColor},
		{"clipboard auto", nil, true, CapabilityRequest{Format: "text", ColorMode: "auto", IconMode: "auto", Clipboard: true}, false, "unicode", ProfileTrueColor},
		{"clipboard forced color", nil, false, CapabilityRequest{Format: "text", ColorMode: "always", IconMode: "nerd", Clipboard: true}, true, "nerd", ProfileTrueColor},
		{"clipboard never icons", nil, true, CapabilityRequest{Format: "text", ColorMode: "auto", IconMode: "never", Clipboard: true}, false, "never", ProfileTrueColor},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evaluator := testEvaluator(test.env, test.tty, nil)
			test.request.Writer = &bytes.Buffer{}
			got, err := evaluator.Evaluate(test.request)
			if err != nil {
				t.Fatal(err)
			}
			if got.ColorEnabled != test.color || got.IconMode != test.icons || got.Profile != test.profile {
				t.Fatalf("capabilities = %#v", got)
			}
		})
	}
}

func TestTerminalPreparationFailureAndRestore(t *testing.T) {
	prepareError := errors.New("vtp failed")
	evaluator := testEvaluator(nil, true, func(io.Writer) (func() error, error) { return nil, prepareError })
	if _, err := evaluator.Evaluate(CapabilityRequest{Format: "text", ColorMode: "always", IconMode: "never", Writer: &bytes.Buffer{}}); err == nil || IsInvalid(err) {
		t.Fatalf("forced error = %v", err)
	}
	auto, err := evaluator.Evaluate(CapabilityRequest{Format: "text", ColorMode: "auto", IconMode: "never", Writer: &bytes.Buffer{}})
	if err != nil || auto.ColorEnabled {
		t.Fatalf("auto = %#v err=%v", auto, err)
	}

	restored := false
	evaluator = testEvaluator(nil, true, func(io.Writer) (func() error, error) { return func() error { restored = true; return nil }, nil })
	forced, err := evaluator.Evaluate(CapabilityRequest{Format: "text", ColorMode: "always", IconMode: "never", Writer: &bytes.Buffer{}})
	if err != nil {
		t.Fatal(err)
	}
	if err := forced.Close(); err != nil || !restored {
		t.Fatalf("restore err=%v restored=%t", err, restored)
	}
}

func TestIconAutoUsesDeclaredNerdCapabilityOnly(t *testing.T) {
	nerdTrue := true
	nerdFalse := false
	tests := []struct {
		name    string
		env     map[string]string
		tty     bool
		request CapabilityRequest
		want    string
	}{
		{"auto absent stdout", nil, true, CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsAuto}, IconsUnicode},
		{"auto absent pipe", nil, false, CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsAuto}, IconsUnicode},
		{"auto absent output", nil, true, CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsAuto, OutputPath: "tree.txt"}, IconsUnicode},
		{"auto absent clipboard", nil, true, CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsAuto, Clipboard: true}, IconsUnicode},
		{"auto absent CI", map[string]string{"CI": "1"}, true, CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsAuto}, IconsUnicode},
		{"auto absent dumb", map[string]string{"TERM": "dumb"}, true, CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsAuto}, IconsUnicode},
		{"auto config true stdout", nil, true, CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsAuto, ConfiguredNerdFont: &nerdTrue}, IconsNerd},
		{"auto config true pipe", nil, false, CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsAuto, ConfiguredNerdFont: &nerdTrue}, IconsNerd},
		{"auto config false", nil, true, CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsAuto, ConfiguredNerdFont: &nerdFalse}, IconsUnicode},
		{"auto env overrides false config", map[string]string{NerdFontEnvironmentVariable: "true"}, true, CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsAuto, ConfiguredNerdFont: &nerdFalse}, IconsNerd},
		{"auto env overrides true config", map[string]string{NerdFontEnvironmentVariable: "false"}, true, CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsAuto, ConfiguredNerdFont: &nerdTrue}, IconsUnicode},
		{"explicit nerd ignores false capability", map[string]string{NerdFontEnvironmentVariable: "0"}, true, CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsNerd, ConfiguredNerdFont: &nerdFalse}, IconsNerd},
		{"explicit unicode ignores true capability", map[string]string{NerdFontEnvironmentVariable: "1"}, true, CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsUnicode, ConfiguredNerdFont: &nerdTrue}, IconsUnicode},
		{"explicit ascii ignores true capability", map[string]string{NerdFontEnvironmentVariable: "1"}, true, CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsASCII, ConfiguredNerdFont: &nerdTrue}, IconsASCII},
		{"explicit never ignores true capability", map[string]string{NerdFontEnvironmentVariable: "1"}, true, CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsNever, ConfiguredNerdFont: &nerdTrue}, IconsNever},
		{"windows terminal without capability stays unicode", map[string]string{"WT_SESSION": "1", "COLORTERM": "truecolor", "TERM": "xterm-256color"}, true, CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsAuto}, IconsUnicode},
		{"machine format stays never", map[string]string{NerdFontEnvironmentVariable: "1"}, true, CapabilityRequest{Format: "json", ColorMode: "always", IconMode: IconsNerd, ConfiguredNerdFont: &nerdTrue}, IconsNever},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			evaluator := testEvaluator(test.env, test.tty, nil)
			test.request.Writer = &bytes.Buffer{}
			got, err := evaluator.Evaluate(test.request)
			if err != nil {
				t.Fatal(err)
			}
			if got.IconMode != test.want {
				t.Fatalf("icon mode = %q, want %q", got.IconMode, test.want)
			}
		})
	}
}

func TestNerdFontEnvironmentParsing(t *testing.T) {
	for _, value := range []string{"1", "true", "TRUE", "yes", "YES", "on", "ON"} {
		got, err := parseNerdFontEnvironment(value)
		if err != nil || !got {
			t.Errorf("%q = %t, %v", value, got, err)
		}
	}
	for _, value := range []string{"0", "false", "FALSE", "no", "NO", "off", "OFF"} {
		got, err := parseNerdFontEnvironment(value)
		if err != nil || got {
			t.Errorf("%q = %t, %v", value, got, err)
		}
	}
	if _, err := parseNerdFontEnvironment("maybe"); err == nil || !IsInvalid(err) || !strings.Contains(err.Error(), `invalid DIRLOOM_NERD_FONT value "maybe"`) {
		t.Fatalf("invalid env = %v", err)
	}
	evaluator := testEvaluator(map[string]string{NerdFontEnvironmentVariable: "maybe"}, true, nil)
	if _, err := evaluator.Evaluate(CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsAuto, Writer: &bytes.Buffer{}}); err == nil || !IsInvalid(err) {
		t.Fatalf("auto with invalid env = %v", err)
	}
	got, err := evaluator.Evaluate(CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsNever, Writer: &bytes.Buffer{}})
	if err != nil || got.IconMode != IconsNever {
		t.Fatalf("explicit mode must ignore invalid env: %#v err=%v", got, err)
	}
	got, err = evaluator.Evaluate(CapabilityRequest{Format: "text", ColorMode: "never", IconMode: IconsUnicode, Writer: &bytes.Buffer{}})
	if err != nil || got.IconMode != IconsUnicode {
		t.Fatalf("unicode must ignore invalid env: %#v err=%v", got, err)
	}
}

func TestUnsupportedIconModeIncludesASCII(t *testing.T) {
	evaluator := testEvaluator(nil, true, nil)
	_, err := evaluator.Evaluate(CapabilityRequest{Format: "text", ColorMode: "never", IconMode: "font", Writer: &bytes.Buffer{}})
	if err == nil || !IsInvalid(err) || !strings.Contains(err.Error(), "ascii") {
		t.Fatalf("unsupported icon mode = %v", err)
	}
}

func testEvaluator(env map[string]string, tty bool, prepare func(io.Writer) (func() error, error)) *Evaluator {
	options := []EvaluatorOption{
		WithEnvironment(func(name string) (string, bool) { value, ok := env[name]; return value, ok }),
		WithTerminalDetection(func(io.Writer) bool { return tty }),
		WithWindowsTerminalCompatibility(false),
	}
	if prepare != nil {
		options = append(options, WithANSIPreparation(prepare))
	}
	return NewEvaluator(options...)
}
