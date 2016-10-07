package gopass

import (
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/crypto/ssh/terminal"
)

type terminalState struct {
	state     *terminal.State
	sttyState string
}

func stty(stdinFd uintptr, args ...string) ([]byte, error) {
	// Grab the current process.
	selfProcess, err := syscall.GetCurrentProcess()
	if err != nil {
		return nil, err
	}

	// Create a copy of the file descriptor, because we'll need to wrap it up in
	// an os.File (in order to pass it to the stty process), and that file will
	// need to be closed.
	var newFD syscall.Handle
	err = syscall.DuplicateHandle(
		selfProcess, syscall.Handle(stdinFd), // Duplicate from this process.
		selfProcess, &newFD, // Into the same process.
		0,    // Don't need to specify access since we duplicate it.
		true, // Make the descriptor inheritable (required for passing to stty).
		syscall.DUPLICATE_SAME_ACCESS, // Duplicate access rights.
	)
	if err != nil {
		return nil, err
	}

	// Wrap our file descriptor copy up in a file, and ensure it's closed when
	// we're done. Go's os module uses the name "/dev/stdin" on all platforms,
	// including Windows, so we use it here as well (it's basically meaningless
	// for our purposes).
	stdin := os.NewFile(uintptr(newFD), "/dev/stdin")
	defer stdin.Close()

	// Run stty.
	sttyProcess := exec.Command("stty", args...)
	sttyProcess.Stdin = stdin
	return sttyProcess.Output()
}

func isPOSIXTerminal(fd uintptr) bool {
	// stty will fail if its standard input is not a terminal, so we just need
	// to check if it exited correctly. If there is no stty command, this isn't
	// a POSIX terminal.
	_, err := stty(fd)
	return err == nil
}

func isTerminal(fd uintptr) bool {
	return terminal.IsTerminal(int(fd)) || isPOSIXTerminal(fd)
}

func makeRaw(fd uintptr) (*terminalState, error) {
	// Handle the standard Windows console.
	if terminal.IsTerminal(int(fd)) {
		state, err := terminal.MakeRaw(int(fd))
		if err != nil {
			return nil, err
		}
		return &terminalState{
			state: state,
		}, err
	}

	// Handle POSIX consoles on Windows.

	// Record the existing state.
	stateBytes, err := stty(fd, "--save")
	if err != nil {
		return nil, err
	}
	state := strings.TrimSpace(string(stateBytes))

	// Put the terminal into raw mode. These settings emulate cfmakeraw, and
	// should be identical to those found in
	// golang.org/x/crypto/ssh/terminal, except there is no need to unset
	// the CSIZE mask because it's implicitly done when setting the new
	// character size.
	rawFlags := []string{
		"-ignbrk", "-brkint", "-parmrk", "-istrip", "-inlcr", "-igncr", "-icrnl", "-ixon",
		"-opost",
		"-echo", "-echonl", "-icanon", "-isig", "-iexten",
		"-parenb",
		"cs8",
	}
	if _, err = stty(fd, rawFlags...); err != nil {
		return nil, err
	}

	// Success.
	return &terminalState{
		sttyState: state,
	}, nil
}

func restore(fd uintptr, oldState *terminalState) error {
	// Handle the standard Windows console.
	if oldState.state != nil {
		return terminal.Restore(int(fd), oldState.state)
	}

	// Handle POSIX console on Windows.
	_, err := stty(fd, oldState.sttyState)
	return err
}
