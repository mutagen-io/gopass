package gopass

import (
	"errors"
	"fmt"
	"io"
	"os"
)

type echoMode uint8

const (
	echoModeNone echoMode = iota
	echoModeMask
	echoModeEcho
)

func (m echoMode) String() string {
	if m == echoModeNone {
		return "none"
	} else if m == echoModeMask {
		return "mask"
	} else if m == echoModeEcho {
		return "echo"
	}
	return "unknown"
}

var defaultGetCh = func() (byte, error) {
	buf := make([]byte, 1)
	if n, err := os.Stdin.Read(buf); n == 0 || err != nil {
		if err != nil {
			return 0, err
		}
		return 0, io.EOF
	}
	return buf[0], nil
}

var (
	maxLength            = 512
	ErrInterrupted       = errors.New("interrupted")
	ErrMaxLengthExceeded = fmt.Errorf("maximum byte limit (%v) exceeded", maxLength)

	// Provide variable so that tests can provide a mock implementation.
	getch = defaultGetCh
)

// getPasswd returns the input read from terminal. It echos according to the
// mode specified.
func getPasswd(mode echoMode) ([]byte, error) {
	var err error
	var pass, bs, mask []byte
	if mode == echoModeMask || mode == echoModeEcho {
		bs = []byte("\b \b")
	}
	if mode == echoModeMask {
		mask = []byte("*")
	}

	if isTerminal(os.Stdin.Fd()) {
		if oldState, err := makeRaw(os.Stdin.Fd()); err != nil {
			return pass, err
		} else {
			defer func() {
				restore(os.Stdin.Fd(), oldState)
				fmt.Println()
			}()
		}
	}

	// Track total bytes read, not just bytes in the password.  This ensures any
	// errors that might flood the console with nil or -1 bytes infinitely are
	// capped.
	var counter int
	for counter = 0; counter <= maxLength; counter++ {
		if v, e := getch(); e != nil {
			err = e
			break
		} else if v == 127 || v == 8 {
			if l := len(pass); l > 0 {
				pass = pass[:l-1]
				fmt.Print(string(bs))
			}
		} else if v == 13 || v == 10 {
			break
		} else if v == 3 {
			err = ErrInterrupted
			break
		} else if v != 0 {
			pass = append(pass, v)
			if mode == echoModeMask {
				fmt.Print(string(mask))
			} else if mode == echoModeEcho {
				fmt.Print(string(v))
			}
		}
	}

	if counter > maxLength {
		err = ErrMaxLengthExceeded
	}

	return pass, err
}

// GetPasswd returns the password read from the terminal without echoing input.
// The returned byte array does not include end-of-line characters.
func GetPasswd() ([]byte, error) {
	return getPasswd(echoModeNone)
}

// GetPasswdMasked returns the password read from the terminal, echoing asterisks.
// The returned byte array does not include end-of-line characters.
func GetPasswdMasked() ([]byte, error) {
	return getPasswd(echoModeMask)
}

// GetPasswdMasked returns the password read from the terminal, echoing input.
// The returned byte array does not include end-of-line characters.
func GetPasswdEchoed() ([]byte, error) {
	return getPasswd(echoModeEcho)
}
