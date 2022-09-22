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
	"time"

	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
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

	if err != nil {
		return "", err
	}

	fmt.Println("Email Sent")
	emailData.Date = time.Now()

	_, err = Collection.InsertOne(ctx, emailData)

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
		"From":    {m.FormatAddress("ramashankar.kumar@gridinfocom.com", "Ramashankar")},
		"To":      emailData.EmailTo,
		"Cc":      emailData.EmailCC,
		"Subject": emailData.EmailSubject,
	})

	m.SetBody("text/plain", emailData.EmailBody)
	// Settings for SMTP server
	d := gomail.NewDialer("smtp-relay.sendinblue.com", 587, "ramashankar.kumar@gridinfocom.com", "vrDYSBXaFb2y6VnE")

	d.TLSConfig = &tls.Config{InsecureSkipVerify: true}
	// Now send E-Mail
	if err := d.DialAndSend(m); err != nil {
		fmt.Println(err)
		return err
	}
	return nil
}

func (e *Connection) SearchFilter(search pojo.Search) ([]*pojo.EmailPojo, error) {
	var searchData []*pojo.EmailPojo

	filter := bson.D{}

	if search.EmailTo != "" {
		filter = append(filter, primitive.E{Key: "email_to", Value: bson.M{"$regex": search.EmailTo}})
	}
	if search.EmailCC != "" {
		filter = append(filter, primitive.E{Key: "email_cc", Value: bson.M{"$regex": search.EmailCC}})
	}
	if search.EmailBCC != "" {
		filter = append(filter, primitive.E{Key: "email_bcc", Value: bson.M{"$regex": search.EmailBCC}})
	}
	if search.EmailSubject != "" {
		filter = append(filter, primitive.E{Key: "email_subject", Value: bson.M{"$regex": search.EmailSubject}})
	}

	t, _ := time.Parse("2006-01-02", search.Date)
	if search.Date != "" {
		filter = append(filter, primitive.E{Key: "date", Value: bson.M{
			"$gte": primitive.NewDateTimeFromTime(t)}})
	}

	result, err := Collection.Find(ctx, filter)

	if err != nil {
		return searchData, err
	}

	for result.Next(ctx) {
		var data pojo.EmailPojo
		err := result.Decode(&data)
		if err != nil {
			return searchData, err
		}
		searchData = append(searchData, &data)
	}

	if searchData == nil {
		return searchData, errors.New("Data Not Found In DB")
	}

	return searchData, nil
}
