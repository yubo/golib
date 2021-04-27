package mail

import (
	"testing"

	gomail "gopkg.in/gomail.v2"
)

func TestSendmail(t *testing.T) {
	m := gomail.NewMessage()

	m.SetHeader("From", m.FormatAddress("alex@example.com", "Alex"))
	m.SetHeader("To", "yubo@yubo.org")
	m.SetHeader("Subject", "Hello!")
	//m.SetBody("text/plain", "Hello!")
	m.Embed("./out.png", gomail.Rename("otp.png"))
	m.SetBody("text/html", `<img src="cid:otp.png" alt="otp" />`)

	d := gomail.NewDialer("mail.yubo.org", 25, "", "")
	if err := d.DialAndSend(m); err != nil {
		panic(err)
	}

}
