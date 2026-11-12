//go:build !unix

package transcode

import (
	"errors"
	"os"
)

// Without SIGSTOP, ffmpeg isn't throttled: it runs ahead to the end of the
// file, and old segments are still deleted as the player moves on.
func pause(*os.Process) error  { return errors.ErrUnsupported }
func resume(*os.Process) error { return errors.ErrUnsupported }
