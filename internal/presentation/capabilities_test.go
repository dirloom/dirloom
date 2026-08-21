package presentation

import (
	"bytes"
	"errors"
	"io"
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
		{"auto pipe", nil, false, CapabilityRequest{Format: "text", ColorMode: "auto", IconMode: "auto"}, false, "never", ProfileTrueColor},
		{"ci", map[string]string{"CI": "1"}, true, CapabilityRequest{Format: "text", ColorMode: "auto", IconMode: "auto"}, false, "never", ProfileANSI16},
		{"dumb", map[string]string{"TERM": "dumb"}, true, CapabilityRequest{Format: "text", ColorMode: "auto", IconMode: "auto"}, false, "never", ProfileANSI16},
		{"output", nil, true, CapabilityRequest{Format: "text", ColorMode: "auto", IconMode: "auto", OutputPath: "tree.txt"}, false, "never", ProfileTrueColor},
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
