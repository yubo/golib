package mail

import (
	"bytes"
	"io/ioutil"
	"os"

	"text/template"

	"github.com/yubo/golib/status"
	"github.com/yubo/golib/util"
	"google.golang.org/grpc/codes"
	gomail "gopkg.in/gomail.v2"
)

type Config struct {
	From     []string `json:"from"`
	Host     string   `json:"host"`
	Port     int      `json:"port"`
	Username string   `json:"username"`
	Password string   `json:"password"`
	TmpDir   string   `json:"tmpDir"`
}

func (p Config) String() string {
	return util.Prettify(p)
}

func (p *Config) Validate() error {
	return nil
}

type MailContext struct {
	*Config
	Dialer  *gomail.Dialer
	Message *gomail.Message
	tmpFile []string
}

func NewMail(cf *Config, contentType string, tpl *template.Template, input interface{}) (*MailContext, error) {
	if cf == nil {
		return nil, status.Errorf(codes.Internal, "mail config is nil ptr")
	}
	buff := &bytes.Buffer{}
	if err := tpl.Execute(buff, input); err != nil {
		return nil, err
	}

	m := gomail.NewMessage()
	if len(cf.From) == 2 {
		m.SetHeader("From", m.FormatAddress(cf.From[0], cf.From[1]))
	} else {
		m.SetHeader("From", cf.From[0])
	}

	m.SetBody(contentType, buff.String())
	d := gomail.NewDialer(cf.Host, cf.Port, cf.Username, cf.Password)

	return &MailContext{
		Config:  cf,
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
