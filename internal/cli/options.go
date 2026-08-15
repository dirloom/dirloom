package cli

import (
	"fmt"
	"strconv"
)

type optionalDepth struct {
	set       bool
	unlimited bool
	value     int
}

func (d *optionalDepth) Set(raw string) error {
	if raw == "unlimited" {
		d.set = true
		d.unlimited = true
		d.value = 0
		return nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return fmt.Errorf("depth must be a non-negative integer or unlimited")
	}
	if value < 0 {
		return fmt.Errorf("depth must be a non-negative integer or unlimited")
	}
	d.set = true
	d.unlimited = false
	d.value = value
	return nil
}

func (d *optionalDepth) String() string {
	if !d.set {
		return ""
	}
	if d.unlimited {
		return "unlimited"
	}
	return strconv.Itoa(d.value)
}

func (*optionalDepth) Type() string {
	return "non-negative integer or unlimited"
}

type options struct {
	preset          string
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

type sourceOptions struct {
	path     string
	noUser   bool
	noConfig bool
}
