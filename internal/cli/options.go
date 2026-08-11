package cli

import (
	"fmt"
	"strconv"
)

type optionalDepth struct {
	set   bool
	value int
}

func (d *optionalDepth) Set(raw string) error {
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fmt.Errorf("depth must be a non-negative integer")
	}
	if value < 0 {
		return fmt.Errorf("depth must be a non-negative integer")
	}
	d.set = true
	d.value = value
	return nil
}

func (d *optionalDepth) String() string {
	if !d.set {
		return ""
	}
	return strconv.Itoa(d.value)
}

func (*optionalDepth) Type() string {
	return "non-negative integer"
}

func (d *optionalDepth) Pointer() *int {
	if !d.set {
		return nil
	}
	value := d.value
	return &value
}

type options struct {
	depth           optionalDepth
	directoriesOnly bool
	includeHidden   bool
	ignorePatterns  []string
	noDefaultIgnore bool
	noGitIgnore     bool
	format          string
	style           string
	output          string
}
