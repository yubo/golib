package mail

import (
	"bytes"
	"fmt"
	"io"
	"io/ioutil"
	"os"

	ht "html/template"

	gomail "gopkg.in/gomail.v2"
)

type Config struct {
	Enabled  bool     `json:"enabled"`
	From     []string `json:"from"`
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	TmpDir   string   `json:"tmpDir"`
}

func NewConfig() *Config {
	return &Config{
		TmpDir: os.TempDir(),
	}
}

func (p *Config) Validate() error {
	if p.TmpDir == "" {
		p.TmpDir = os.TempDir()
	}
	if !isDir(p.TmpDir) {
		return fmt.Errorf("mail.tmpdir %s not exist", p.TmpDir)
	}

	return nil
}

type MailContext struct {
	*Config
	Dialer  *gomail.Dialer
	Message *gomail.Message
	tmpFile []string
}

type Executer interface {
	Execute(wr io.Writer, data interface{}) error
}

func (p *Config) NewMail(tpl Executer, input interface{}) (*MailContext, error) {
	if p == nil {
		return nil, fmt.Errorf("mail config is nil ptr")
	}
	if !p.Enabled {
		return nil, fmt.Errorf("mail is not enabled")
	}

	buff := &bytes.Buffer{}
	var contentType string

	if tpl_, ok := tpl.(*ht.Template); ok {
		if err := tpl_.Execute(buff, input); err != nil {
			return nil, err
		}
		contentType = "text/html"
	} else {
		if err := tpl.Execute(buff, input); err != nil {
			return nil, err
		}
		contentType = "text/plain"
	}

	m := gomail.NewMessage()

	if len(p.From) == 2 {
		m.SetHeader("From", m.FormatAddress(p.From[0], p.From[1]))
	} else {
		m.SetHeader("From", p.From[0])
	}

	m.SetBody(contentType, buff.String())
	d := gomail.NewDialer(p.Host, p.Port, p.Username, p.Password)

	return &MailContext{
		Config:  p,
		Dialer:  d,
		Message: m,
	}, nil
}

func (p *MailContext) EmbedBuffer(name string, body []byte) error {
	tmpFile, err := ioutil.TempFile(p.TmpDir, "mail_")
	if err != nil {
		return err
	}
	defer tmpFile.Close()

	filename := tmpFile.Name()
	p.tmpFile = append(p.tmpFile, filename)

	if _, err := tmpFile.Write(body); err != nil {
		return err
	}

	p.Message.Embed(filename, gomail.Rename(name))
	return nil
}

func (p *MailContext) DialAndSend() error {
	defer func() {
		for _, filename := range p.tmpFile {
			os.Remove(filename)
		}
	}()

	return p.Dialer.DialAndSend(p.Message)
}

func (p *MailContext) SetHeader(field string, value ...string) {
	p.Message.SetHeader(field, value...)
}

/*
	m.SetHeader("From", m.FormatAddress("alex@example.com", "Alex"))
	m.SetHeader("To", "yubo@yubo.org")
	m.SetHeader("Subject", "Hello!")
	m.Embed("./out.png", gomail.Rename("otp.png"))
	m.SetBody("text/html", `<img src="cid:otp.png" alt="otp" />`)
	d := gomail.NewDialer("mail.yubo.org", 25, "", "")
*/

func isDir(file string) bool {
	f, e := os.Stat(file)
	if e != nil {
		return false
	}
	return f.IsDir()
}
