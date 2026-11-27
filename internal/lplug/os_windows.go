//go:build windows

package lplug

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// trusted: Windows ACLs aren't checked; restrict the config dir yourself.
func trusted(os.FileInfo) (bool, string) { return true, "" }

func isExecutable(path string, _ os.FileInfo) bool {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".exe", ".bat", ".cmd":
		return true
	}
	return false
}

func killGroup(*exec.Cmd) {}
