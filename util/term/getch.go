package term

import (
	"errors"
	"os"

	"github.com/yubo/golib/util/term/moby/term"
)

const (
	_ = iota
	TERM_CODE_UP
	TERM_CODE_DOWN
	TERM_CODE_RIGHT
	TERM_CODE_LEFT
)

func Getch() (ascii byte, keyCode int, err error) {
	var (
		numRead int
		buff    = make([]byte, 3)
	)

	inFd, isTerminal := term.GetFdInfo(os.Stdin)
	if !isTerminal {
		if f, err := os.Open("/dev/tty"); err == nil {
			defer f.Close()
			inFd = f.Fd()
			isTerminal = term.IsTerminal(inFd)
		}
	}
	if !isTerminal {
		err = errors.New("fd is not terminal")
		return
	}

	state, err := term.MakeRaw(inFd)
	defer term.RestoreTerminal(inFd, state)

	if numRead, err = os.Stdin.Read(buff); err != nil {
		return
	}

	if numRead == 1 {
		ascii = buff[0]
		return
	}

	if numRead == 3 && buff[0] == 27 && buff[1] == 91 {
		// Three-character control sequence, beginning with "ESC-[".

		// Since there are no ASCII codes for arrow keys, we use
		// Javascript key codes.
		switch buff[2] {
		case 65:
			keyCode = TERM_CODE_UP
		case 66:
			keyCode = TERM_CODE_DOWN
		case 67:
			keyCode = TERM_CODE_RIGHT
		case 68:
			keyCode = TERM_CODE_LEFT
		}
		return
	}

	// Two characters read??
	return
}
