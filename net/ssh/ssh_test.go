package ssh

import (
	"io/ioutil"
	"os"
	"os/user"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

var (
	// cat ~/.ssh/id_rsa.pub >> ~/.ssh/authorized_keys
	testConfig1 = &ClientConfig{
		Addr:       "127.0.0.1:22",
		User:       username(),
		PrivateKey: privateKey(),
	}
	testConfig2 = &ClientConfig{
		Addr:       "localhost:22",
		User:       username(),
		PrivateKey: privateKey(),
	}
)

func TestRef(t *testing.T) {
	c1, err := Dial(testConfig1, time.Second, nil)
	if err != nil {
		t.Logf("ignore dial error %s", err)
		return
	}
	require.Equalf(t, 1, c1.ref, "dial")

	c2, err := Dial(testConfig1, 10*time.Second, nil)
	require.Empty(t, err)
	require.Equalf(t, 2, c1.ref, "dial")

	s1, err := c1.NewSession()
	require.Emptyf(t, err, "session")

	s2, err := c2.NewSession()
	require.Emptyf(t, err, "session")

	err = s1.Run("true")
	require.Emptyf(t, err, "session")
	s1.Close()

	err = s2.Run("true")
	require.Emptyf(t, err, "session run")
	s2.Close()

	err = c1.Close()
	require.Emptyf(t, err, "close")
	require.Equalf(t, 1, c1.ref, "close")

	err = c2.Close()
	require.Emptyf(t, err, "close")
	require.Equalf(t, 0, c1.ref, "close")
}

func TestProxy(t *testing.T) {
	c1, err := Dial(testConfig1, time.Second, nil)
	if err != nil {
		t.Logf("ignore dial error %s", err)
		return
	}
	require.Equalf(t, 1, c1.ref, "dial")
	defer c1.Close()

	c2, err := Dial(testConfig2, 10*time.Second, testConfig1)
	require.Empty(t, err)
	require.Equalf(t, 1, c2.ref, "dial")
	defer c2.Close()

	s2, err := c2.NewSession()
	require.Emptyf(t, err, "session")
	defer s2.Close()

	err = s2.Run("true")
	require.Emptyf(t, err, "session run")
}

func TestReconnect(t *testing.T) {
	// s1
	c1, err := Dial(testConfig1, time.Second, nil)
	if err != nil {
		t.Logf("ignore dial error %s", err)
		return
	}
	s1, _ := c1.NewSession()
	defer s1.Close()
	defer c1.Close()

	c1.Conn().Close() // force close sshclient
	require.Equalf(t, 1, c1.ref, "dial")

	// s2
	c2, _ := Dial(testConfig1, 10*time.Second, nil)
	require.Equalf(t, 2, c2.ref, "dial")

	_, err = c2.NewSession()
	require.NotEmptyf(t, err, "session")

	err = c2.Reconnect()
	require.Equalf(t, 2, c2.ref, "reconnect")

	s2, err := c2.NewSession()
	require.Emptyf(t, err, "session")
	err = s2.Run("true")
	require.Emptyf(t, err, "session run")
	defer s2.Close()
	defer c2.Close()

}

func TestPostCheck(t *testing.T) {
	require.Equalf(t, 0, len(clients), "clients len")
}

func username() string {
	var username string
	if user, err := user.Current(); err == nil {
		username = user.Username
	} else {
		// user.Current() currently requires cgo. If an error is
		// returned attempt to get the username from the environment.
		username = os.Getenv("USER")
	}
	if username == "" {
		panic("Unable to get username")
	}
	return username
}

func privateKey() string {
	b, _ := ioutil.ReadFile(os.Getenv("HOME") + "/.ssh/id_rsa")
	return string(b)
}
