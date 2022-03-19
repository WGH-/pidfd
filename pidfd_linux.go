//go:build linux

package pidfd

import (
	"fmt"
	"os"

	"github.com/mdlayher/socket"
	"golang.org/x/sys/unix"
)

// esrch is the "no such process" errno.
var esrch = unix.ESRCH

// A conn backs File for Linux pidfds. We can use socket.Conn directly on Linux
// to implement most of the necessary methods.
type conn = socket.Conn

// open opens a pidfd File.
func open(pid int) (*File, error) {
	// Open nonblocking: we always use asynchronous I/O anyway with
	// *socket.Conn.
	//
	// TODO(mdlayher): plumb in more pidfd_open flags if it ever makes sense to
	// do so.
	fd, err := unix.PidfdOpen(pid, unix.PIDFD_NONBLOCK)
	if err != nil {
		// No FD to annotate the error yet.
		return nil, &Error{Err: err}
	}

	c, err := socket.New(fd, "pidfd")
	if err != nil {
		return nil, err
	}

	rc, err := c.SyscallConn()
	if err != nil {
		return nil, err
	}

	return &File{
		c:  c,
		rc: rc,
	}, nil
}

// sendSignal signals the process referred to by File.
func (f *File) sendSignal(signal os.Signal) error {
	ssig, ok := signal.(unix.Signal)
	if !ok {
		return fmt.Errorf("pidfd: invalid signal type for File.SendSignal: %T", signal)
	}

	// From pidfd_send_signal(2):
	//
	// "If the info argument is a NULL pointer, this is equivalent to specifying
	// a pointer to a siginfo_t buffer whose fields match the values that are
	// implicitly supplied when a signal is sent using kill(2)"
	//
	// "The flags argument is reserved for future use; currently, this argument
	// must be specified as 0."
	return f.wrap(f.c.PidfdSendSignal(ssig, nil, 0))
}

// wrap annotates and returns an *Error with File metadata. If err is nil, wrap
// is a no-op.
func (f *File) wrap(err error) error {
	if err == nil {
		return nil
	}

	// Best effort.
	var fd int
	_ = f.rc.Control(func(cfd uintptr) {
		fd = int(cfd)
	})

	return &Error{
		FD:  fd,
		Err: err,
	}
}