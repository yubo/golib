package ssh

import (
	"net"
	"sync"
	"time"

	"github.com/yubo/golib/util"
	"golang.org/x/crypto/ssh"
)

var (
	lock    sync.RWMutex
	clients = map[ClientConfig]*Client{}
)

type ClientConfig struct {
	Addr       string
	User       string
	PrivateKey string
}

func (p ClientConfig) String() string {
	return util.Prettify(p)
}

type Client struct {
	conn        *ssh.Client
	Config      *ClientConfig
	ProxyConfig *ClientConfig
	Timeout     time.Duration

	ref int
}

func Dial(config *ClientConfig, timeout time.Duration, proxyConfig *ClientConfig) (*Client, error) {
	return Get(config, timeout, proxyConfig)
}

func Get(config *ClientConfig, timeout time.Duration, proxyConfig *ClientConfig) (*Client, error) {
	lock.Lock()
	defer lock.Unlock()

	return get(config, timeout, proxyConfig)
}

func get(config *ClientConfig, timeout time.Duration, proxyConfig *ClientConfig) (*Client, error) {
	if cli, ok := clients[*config]; ok {
		cli.ref++
		return cli, nil
	}

	cli := &Client{
		Config:      config,
		ProxyConfig: proxyConfig,
		Timeout:     timeout,
		ref:         1,
	}

	if err := cli.dial(); err != nil {
		return nil, err
	}

	clients[*config] = cli
	return cli, nil
}

func (p *Client) NewSession() (*ssh.Session, error) {
	return p.Conn().NewSession()
}

func (p *Client) TryNewSession() (sess *ssh.Session, err error) {
	lock.RLock()
	defer lock.RUnlock()

	if sess, err = p.conn.NewSession(); err == nil {
		return
	}

	if err = p.reconnect(); err != nil {
		return nil, err
	}
	return p.conn.NewSession()

}

func (p *Client) Conn() *ssh.Client {
	lock.RLock()
	defer lock.RUnlock()
	return p.conn
}

func (p *Client) Close() error {
	return p.Put()
}

func (p *Client) DelayClose(second int) error {
	time.AfterFunc(time.Duration(second)*time.Second,
		func() { p.Put() })
	return nil
}

func (p *Client) Put() error {
	lock.Lock()
	defer lock.Unlock()

	return p.put()
}

func (p *Client) put() error {
	if p.ProxyConfig != nil {
		if proxy, ok := clients[*p.ProxyConfig]; ok {
			proxy.put()
		}
	}

	p.ref--
	if p.ref > 0 {
		return nil
	}

	delete(clients, *p.Config)
	p.conn.Close()
	return nil

}

func (p *Client) Reconnect() error {
	lock.Lock()
	defer lock.Unlock()

	return p.reconnect()
}

func (p *Client) reconnect() error {
	p.conn.Close()
	return p.dial()
}

// unthread-safe
func (p *Client) dial() error {
	var conn net.Conn
	var err error

	cf := p.Config
	if p.ProxyConfig != nil {
		proxy, err := get(p.ProxyConfig, p.Timeout, nil)
		if err != nil {
			return err
		}
		conn, err = proxy.conn.Dial("tcp", cf.Addr)
		if err != nil {
			err = proxy.reconnect()
			if err != nil {
				return err
			}
			conn, err = proxy.conn.Dial("tcp", cf.Addr)
			if err != nil {
				return err
			}
		}
	} else {
		conn, err = net.DialTimeout("tcp", cf.Addr, p.Timeout)
	}
	if err != nil {
		return err
	}

	key, err := ssh.ParsePrivateKey([]byte(cf.PrivateKey))
	if err != nil {
		return err
	}
	config := &ssh.ClientConfig{
		User:            cf.User,
		Auth:            []ssh.AuthMethod{ssh.PublicKeys(key)},
		Timeout:         p.Timeout,
		HostKeyCallback: ssh.InsecureIgnoreHostKey(),
	}

	c, chans, reqs, err := ssh.NewClientConn(conn, cf.Addr, config)
	if err != nil {
		return err
	}

	if p.conn != nil {
		// try to close old conn
		p.conn.Close()
	}
	p.conn = ssh.NewClient(c, chans, reqs)
	return nil
}
