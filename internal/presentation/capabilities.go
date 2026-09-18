package presentation

import (
	"fmt"
	"io"
	"os"
	"runtime"
	"strings"

	"github.com/dirloom/dirloom/internal/outputformat"
	"golang.org/x/term"
)

// CapabilityRequest describes the selected output and the current destination.
type CapabilityRequest struct {
	Format             string
	ColorMode          string
	IconMode           string
	ColorExplicitCLI   bool
	ConfiguredNerdFont *bool
	OutputPath         string
	Clipboard          bool
	Writer             io.Writer
}

// Capabilities is the deterministic terminal presentation decision.
type Capabilities struct {
	ColorEnabled bool
	IconMode     string
	Profile      ColorProfile
	restore      func() error
}

// Close restores any terminal mode changed during capability preparation.
func (capabilities Capabilities) Close() error {
	if capabilities.restore == nil {
		return nil
	}
	return capabilities.restore()
}

// Evaluator resolves terminal capabilities with injectable host behavior.
type Evaluator struct {
	lookupEnv func(string) (string, bool)
	isTTY     func(io.Writer) bool
	prepare   func(io.Writer) (func() error, error)
	windows   bool
}

// EvaluatorOption overrides one host capability, primarily for tests.
type EvaluatorOption func(*Evaluator)

// WithEnvironment injects environment lookup.
func WithEnvironment(lookup func(string) (string, bool)) EvaluatorOption {
	return func(evaluator *Evaluator) { evaluator.lookupEnv = lookup }
}

// WithTerminalDetection injects TTY detection.
func WithTerminalDetection(detect func(io.Writer) bool) EvaluatorOption {
	return func(evaluator *Evaluator) { evaluator.isTTY = detect }
}

// WithANSIPreparation injects platform terminal preparation.
func WithANSIPreparation(prepare func(io.Writer) (func() error, error)) EvaluatorOption {
	return func(evaluator *Evaluator) { evaluator.prepare = prepare }
}

// WithWindowsTerminalCompatibility overrides modern Windows truecolor detection.
func WithWindowsTerminalCompatibility(enabled bool) EvaluatorOption {
	return func(evaluator *Evaluator) { evaluator.windows = enabled }
}

// NewEvaluator creates a host-backed capability evaluator.
func NewEvaluator(options ...EvaluatorOption) *Evaluator {
	evaluator := &Evaluator{
		lookupEnv: os.LookupEnv,
		isTTY: func(writer io.Writer) bool {
			file, ok := writer.(*os.File)
			return ok && term.IsTerminal(int(file.Fd()))
		},
		prepare: func(writer io.Writer) (func() error, error) {
			file, ok := writer.(*os.File)
			if !ok {
				return func() error { return nil }, nil
			}
			return prepareANSI(file)
		},
		windows: runtime.GOOS == "windows",
	}
	for _, option := range options {
		option(evaluator)
	}
	return evaluator
}

// Evaluate resolves auto modes, NO_COLOR, color depth, and platform setup.
func (evaluator *Evaluator) Evaluate(request CapabilityRequest) (Capabilities, error) {
	if !outputformat.UsesPresentation(request.Format) {
		return Capabilities{IconMode: IconsNever, Profile: ProfileTrueColor}, nil
	}
	tty := !request.Clipboard && request.OutputPath == "" && evaluator.isTTY(request.Writer)
	autoEligible := tty && request.OutputPath == "" && evaluator.env("CI") == "" && evaluator.env("TERM") != "dumb"
	colorEnabled := false
	switch request.ColorMode {
	case ColorNever:
	case ColorAlways:
		colorEnabled = true
	case ColorAuto:
		colorEnabled = autoEligible
	default:
		return Capabilities{}, invalidf("unsupported color mode %q (expected never, always, or auto)", request.ColorMode)
	}
	if evaluator.env("NO_COLOR") != "" && (!request.ColorExplicitCLI || request.ColorMode != ColorAlways) {
		colorEnabled = false
	}
	iconMode, err := evaluator.resolveIconMode(request)
	if err != nil {
		return Capabilities{}, err
	}
	result := Capabilities{ColorEnabled: colorEnabled, IconMode: iconMode, Profile: evaluator.profile(tty)}
	if !colorEnabled || !tty {
		return result, nil
	}
	restore, err := evaluator.prepare(request.Writer)
	if err != nil {
		if request.ColorMode == ColorAlways {
			return Capabilities{}, fmt.Errorf("enable ANSI terminal processing: %w", err)
		}
		result.ColorEnabled = false
		return result, nil
	}
	result.restore = restore
	return result, nil
}

func (evaluator *Evaluator) env(name string) string {
	value, _ := evaluator.lookupEnv(name)
	return value
}

func (evaluator *Evaluator) resolveIconMode(request CapabilityRequest) (string, error) {
	switch request.IconMode {
	case IconsNever:
		return IconsNever, nil
	case IconsASCII:
		return IconsASCII, nil
	case IconsUnicode:
		return IconsUnicode, nil
	case IconsNerd:
		return IconsNerd, nil
	case IconsAuto:
		nerdFont, err := evaluator.declaredNerdFont(request)
		if err != nil {
			return "", err
		}
		if nerdFont {
			return IconsNerd, nil
		}
		return IconsUnicode, nil
	default:
		return "", invalidf("unsupported icon mode %q (expected never, ascii, unicode, nerd, or auto)", request.IconMode)
	}
}

func (evaluator *Evaluator) declaredNerdFont(request CapabilityRequest) (bool, error) {
	if value, ok := evaluator.lookupEnv(NerdFontEnvironmentVariable); ok && strings.TrimSpace(value) != "" {
		return parseNerdFontEnvironment(value)
	}
	if request.ConfiguredNerdFont != nil {
		return *request.ConfiguredNerdFont, nil
	}
	return false, nil
}

func parseNerdFontEnvironment(value string) (bool, error) {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "1", "true", "yes", "on":
		return true, nil
	case "0", "false", "no", "off":
		return false, nil
	default:
		return false, invalidf("invalid %s value %q\nexpected one of: 1, true, yes, on, 0, false, no, off", NerdFontEnvironmentVariable, value)
	}
}

func (evaluator *Evaluator) profile(tty bool) ColorProfile {
	colorTerm := strings.ToLower(evaluator.env("COLORTERM"))
	if colorTerm == "truecolor" || colorTerm == "24bit" || evaluator.windows && tty {
		return ProfileTrueColor
	}
	if strings.Contains(strings.ToLower(evaluator.env("TERM")), "256color") {
		return ProfileANSI256
	}
	if !tty {
		return ProfileTrueColor
	}
	return ProfileANSI16
}
