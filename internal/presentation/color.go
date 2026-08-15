package presentation

import (
	"fmt"
	"strconv"
	"strings"
)

// ColorProfile selects the ANSI color representation emitted by Dirloom.
type ColorProfile string

const (
	ProfileTrueColor ColorProfile = "truecolor"
	ProfileANSI256   ColorProfile = "ansi-256"
	ProfileANSI16    ColorProfile = "ansi-16"
)

type colorKind int

const (
	colorDefault colorKind = iota
	colorANSI
	colorRGB
)

type colorSpec struct {
	kind      colorKind
	ansiIndex int
	r, g, b   int
}

var ansiNames = map[string]int{
	"black": 0, "red": 1, "green": 2, "yellow": 3,
	"blue": 4, "magenta": 5, "cyan": 6, "white": 7,
	"bright-black": 8, "bright-red": 9, "bright-green": 10, "bright-yellow": 11,
	"bright-blue": 12, "bright-magenta": 13, "bright-cyan": 14, "bright-white": 15,
}

var ansiRGB = [16][3]int{
	{0, 0, 0}, {128, 0, 0}, {0, 128, 0}, {128, 128, 0},
	{0, 0, 128}, {128, 0, 128}, {0, 128, 128}, {192, 192, 192},
	{128, 128, 128}, {255, 0, 0}, {0, 255, 0}, {255, 255, 0},
	{0, 0, 255}, {255, 0, 255}, {0, 255, 255}, {255, 255, 255},
}

func parseLiteralColor(value string) (colorSpec, error) {
	if value == "default" {
		return colorSpec{kind: colorDefault}, nil
	}
	if strings.HasPrefix(value, "ansi:") {
		name := strings.TrimPrefix(value, "ansi:")
		index, ok := ansiNames[name]
		if !ok {
			return colorSpec{}, fmt.Errorf("unsupported ANSI color %q", value)
		}
		rgb := ansiRGB[index]
		return colorSpec{kind: colorANSI, ansiIndex: index, r: rgb[0], g: rgb[1], b: rgb[2]}, nil
	}
	if len(value) == 7 && value[0] == '#' {
		number, err := strconv.ParseUint(value[1:], 16, 24)
		if err == nil {
			return colorSpec{kind: colorRGB, r: int(number >> 16), g: int(number >> 8 & 0xff), b: int(number & 0xff)}, nil
		}
	}
	return colorSpec{}, fmt.Errorf("unsupported color %q (expected default, ansi:<name>, or #RRGGBB)", value)
}

func colorCode(color colorSpec, profile ColorProfile) string {
	if color.kind == colorDefault {
		return "39"
	}
	if color.kind == colorANSI {
		return ansiCode(color.ansiIndex)
	}
	switch profile {
	case ProfileANSI16:
		return ansiCode(nearestANSI16(color.r, color.g, color.b))
	case ProfileANSI256:
		return fmt.Sprintf("38;5;%d", nearestANSI256(color.r, color.g, color.b))
	default:
		return fmt.Sprintf("38;2;%d;%d;%d", color.r, color.g, color.b)
	}
}

func ansiCode(index int) string {
	if index < 8 {
		return strconv.Itoa(30 + index)
	}
	return strconv.Itoa(90 + index - 8)
}

func nearestANSI16(r, g, b int) int {
	best, bestDistance := 0, int(^uint(0)>>1)
	for index, candidate := range ansiRGB {
		distance := squared(r-candidate[0]) + squared(g-candidate[1]) + squared(b-candidate[2])
		if distance < bestDistance {
			best, bestDistance = index, distance
		}
	}
	return best
}

func nearestANSI256(r, g, b int) int {
	best := nearestANSI16(r, g, b)
	base := ansiRGB[best]
	bestDistance := squared(r-base[0]) + squared(g-base[1]) + squared(b-base[2])
	steps := [6]int{0, 95, 135, 175, 215, 255}
	for red := 0; red < 6; red++ {
		for green := 0; green < 6; green++ {
			for blue := 0; blue < 6; blue++ {
				distance := squared(r-steps[red]) + squared(g-steps[green]) + squared(b-steps[blue])
				if distance < bestDistance {
					bestDistance = distance
					best = 16 + 36*red + 6*green + blue
				}
			}
		}
	}
	for index := 0; index < 24; index++ {
		level := 8 + 10*index
		distance := squared(r-level) + squared(g-level) + squared(b-level)
		if distance < bestDistance {
			bestDistance = distance
			best = 232 + index
		}
	}
	return best
}

func squared(value int) int { return value * value }

func ansiStyle(value string, color colorSpec, styles []string, profile ColorProfile) string {
	codes := make([]string, 0, len(styles)+1)
	if color.kind != colorDefault || len(styles) > 0 {
		codes = append(codes, colorCode(color, profile))
	}
	for _, style := range styles {
		switch style {
		case "bold":
			codes = append(codes, "1")
		case "dim":
			codes = append(codes, "2")
		case "italic":
			codes = append(codes, "3")
		case "underline":
			codes = append(codes, "4")
		}
	}
	if len(codes) == 0 {
		return value
	}
	return "\x1b[" + strings.Join(codes, ";") + "m" + value + "\x1b[0m"
}
