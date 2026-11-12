//go:build unix

package transcode

import (
	"os"
	"syscall"
)

// pause and resume stop and continue ffmpeg, which throttles it without
// losing its progress.
func pause(p *os.Process) error  { return p.Signal(syscall.SIGSTOP) }
func resume(p *os.Process) error { return p.Signal(syscall.SIGCONT) }
