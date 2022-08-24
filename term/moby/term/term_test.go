//go:build !windows
// +build !windows

package term

import (
	"io/ioutil"
	"os"
	"testing"

	"github.com/google/go-cmp/cmp"
	"github.com/stretchr/testify/require"
	"github.com/yubo/golib/term/creack/pty"
)

func newTtyForTest(t *testing.T) (*os.File, *os.File) {
	t.Helper()
	pty, tty, err := pty.Open()
	if err != nil {
		t.Fatalf("error creating pty: %v", err)
	}

	return pty, tty
}

func newTempFile() (*os.File, error) {
	return ioutil.TempFile(os.TempDir(), "temp")
}

func TestGetWinsize(t *testing.T) {
	pty, tty := newTtyForTest(t)
	defer pty.Close()
	defer tty.Close()
	winSize, err := GetWinsize(tty.Fd())
	require.NoError(t, err)
	require.True(t, winSize != nil)

	newSize := Winsize{Width: 200, Height: 200, x: winSize.x, y: winSize.y}
	err = SetWinsize(tty.Fd(), &newSize)
	require.NoError(t, err)
	winSize, err = GetWinsize(tty.Fd())
	require.NoError(t, err)
	require.Equal(t, *winSize, newSize)
}

var cmpWinsize = cmp.AllowUnexported(Winsize{})

func TestSetWinsize(t *testing.T) {
	pty, tty := newTtyForTest(t)
	defer pty.Close()
	defer tty.Close()
	winSize, err := GetWinsize(tty.Fd())
	require.NoError(t, err)
	require.True(t, winSize != nil)
	newSize := Winsize{Width: 200, Height: 200, x: winSize.x, y: winSize.y}
	err = SetWinsize(tty.Fd(), &newSize)
	require.NoError(t, err)
	winSize, err = GetWinsize(tty.Fd())
	require.NoError(t, err)
	require.Equal(t, *winSize, newSize)
}

func TestGetFdInfo(t *testing.T) {
	pty, tty := newTtyForTest(t)
	defer pty.Close()
	defer tty.Close()
	inFd, isTerminal := GetFdInfo(tty)
	require.Equal(t, inFd, tty.Fd())
	require.Equal(t, isTerminal, true)
	tmpFile, err := newTempFile()
	require.NoError(t, err)
	defer tmpFile.Close()
	inFd, isTerminal = GetFdInfo(tmpFile)
	require.Equal(t, inFd, tmpFile.Fd())
	require.Equal(t, isTerminal, false)
}

func TestIsTerminal(t *testing.T) {
	pty, tty := newTtyForTest(t)
	defer pty.Close()
	defer tty.Close()
	isTerminal := IsTerminal(tty.Fd())
	require.Equal(t, isTerminal, true)
	tmpFile, err := newTempFile()
	require.NoError(t, err)
	defer tmpFile.Close()
	isTerminal = IsTerminal(tmpFile.Fd())
	require.Equal(t, isTerminal, false)
}

func TestSaveState(t *testing.T) {
	pty, tty := newTtyForTest(t)
	defer pty.Close()
	defer tty.Close()
	state, err := SaveState(tty.Fd())
	require.NoError(t, err)
	require.True(t, state != nil)
	pty, tty = newTtyForTest(t)
	defer pty.Close()
	defer tty.Close()
	err = RestoreTerminal(tty.Fd(), state)
	require.NoError(t, err)
}

func TestDisableEcho(t *testing.T) {
	pty, tty := newTtyForTest(t)
	defer pty.Close()
	defer tty.Close()
	state, err := SetRawTerminal(tty.Fd())
	defer RestoreTerminal(tty.Fd(), state)
	require.NoError(t, err)
	require.True(t, state != nil)
	err = DisableEcho(tty.Fd(), state)
	require.NoError(t, err)
}
