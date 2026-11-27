//go:build !windows

package lplug

import (
	"os"
	"os/exec"
	"syscall"
)

// trusted: owned by you (or root) and not writable by group or others, so
// nobody else can change what runs.
func trusted(fi os.FileInfo) (bool, string) {
	if st, ok := fi.Sys().(*syscall.Stat_t); ok {
		if uid := int(st.Uid); uid != os.Getuid() && uid != 0 {
			return false, "not owned by you"
		}
	}
	if fi.Mode().Perm()&0o022 != 0 {
		return false, "writable by group or others"
	}
	return true, ""
}

func isExecutable(_ string, fi os.FileInfo) bool { return fi.Mode().Perm()&0o111 != 0 }

// killGroup runs the program in its own process group and kills the whole
// group on timeout, so children (like `sleep` in a shell script) die too.
func killGroup(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	cmd.Cancel = func() error { return syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL) }
}
