package mail

import (
	"encoding/base64"
	"fmt"
	"log"
	"net/smtp"
)

func Send(to string, body string) {
	from := "dimitr1stepanov@yandex.ru"
	password := "qvgepebdkenqvtbr"
	smtpHost := "smtp.yandex.ru"
	smtpPort := "587"

	auth := smtp.PlainAuth("", from, password, smtpHost)

	title := "authorization code"

	header := make(map[string]string)
	header["From"] = from
	header["To"] = to
	header["Subject"] = title
	header["MIME-Version"] = "1.0"
	header["Content-Type"] = "text/plain; charset=\"utf-8\""
	header["Content-Transfer-Encoding"] = "base64"

	message := ""
	for k, v := range header {
		message += fmt.Sprintf("%s: %s\r\n", k, v)
	}
	message += "\r\n" + base64.StdEncoding.EncodeToString([]byte(body))

	err := smtp.SendMail(smtpHost+":"+smtpPort, auth, from, []string{to}, []byte(message))
	if err != nil {
		log.Fatal(err)
	}
}
