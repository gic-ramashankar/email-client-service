package service

import (
	"context"
	"crypto/tls"
	"demo/pojo"
	"errors"
	"fmt"
	"log"
	"net/smtp"
	"strings"

	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	gomail "gopkg.in/mail.v2"
)

type Connection struct {
	Server     string
	Database   string
	Collection string
}

var Collection *mongo.Collection
var ctx = context.TODO()

func (e *Connection) Connect() {
	clientOptions := options.Client().ApplyURI(e.Server)
	client, err := mongo.Connect(ctx, clientOptions)
	if err != nil {
		log.Fatal(err)
	}

	err = client.Ping(ctx, nil)
	if err != nil {
		log.Fatal(err)
	}

	Collection = client.Database(e.Database).Collection(e.Collection)
}

func (e *Connection) SendEmail(emailData pojo.EmailPojo) (string, error) {

	err := sendMail2(emailData)
	fmt.Println(err)
	if err != nil {
		return "", err
	}
	fmt.Println("Email Sent")

	insert, err := Collection.InsertOne(ctx, emailData)
	fmt.Println(insert)
	if err != nil {
		return "", errors.New("Unable To Insert New Record")
	}
	return "Email Sent Successfully", nil
}

func mail(emailData pojo.EmailPojo) error {
	from := "ranveer.singh@gridinfocom.com"
	password := "xsmtpsib-2af236a5040e4b54343f4fe5b59826e9e7588b2e33d160249ab60a3060fbf348-LAnNRJT0sxHcPqYr"

	toEmailAddress := emailData.EmailTo
	host := "smtp-relay.sendinblue.com"
	port := "587"
	address := host + ":" + port
	msg := ComposeMsg(emailData)

	auth := smtp.PlainAuth("", from, password, host)
	fmt.Println(auth)

	err := smtp.SendMail(address, auth, from, toEmailAddress, []byte(msg))
	return err
}
func ComposeMsg(emailData pojo.EmailPojo) string {

	// empty string
	msg := ""
	// set sender
	msg += fmt.Sprintf("From: %s\r\n", emailData.EmailTo)
	// if more than 1 recipient
	if len(emailData.EmailTo) > 0 {
		msg += fmt.Sprintf("Cc: %s\r\n", strings.Join(emailData.EmailCC, ";"))
	}
	// add subject
	msg += fmt.Sprintf("Subject: %s\r\n", emailData.EmailSubject)
	// add mail body
	msg += fmt.Sprintf("\r\n%s\r\n", emailData.EmailBody)
	return msg
}

func sendMail2(emailData pojo.EmailPojo) error {
	m := gomail.NewMessage()
	m.SetHeaders(map[string][]string{
		"From":    {m.FormatAddress("ranveer.singh@gridinfocom.com", "Ranveer")},
		"To":      emailData.EmailTo,
		"Cc":      emailData.EmailCC,
		"Subject": emailData.EmailSubject,
	})

	m.SetBody("text/plain", emailData.EmailBody)
	// Settings for SMTP server
	d := gomail.NewDialer("smtp-relay.sendinblue.com", 587, "ranveer.singh@gridinfocom.com", "xsmtpsib-2af236a5040e4b54343f4fe5b59826e9e7588b2e33d160249ab60a3060fbf348-LAnNRJT0sxHcPqYr")

	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	// Now send E-Mail
	if err := d.DialAndSend(m); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}
