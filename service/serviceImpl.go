package service

import (
	"context"
	"crypto/tls"
	"demo/pojo"
	"errors"
	"fmt"
	"io"
	"log"
	"mime/multipart"
	"os"
	"time"

	"github.com/unidoc/unipdf/v3/common/license"
	"github.com/unidoc/unipdf/v3/creator"
	"github.com/unidoc/unipdf/v3/model"
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

const maxUploadSize = 10 * 1024 * 1024 // 10 mb
const dir = "data/download/"

var fileName string
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

	err = license.SetMeteredKey("72c4ab06d023bbc8b2e186d089f9e052654afea32b75141f39c7dc1ab3b108ca")
	if err != nil {
		log.Fatal(err)
	}

	Collection = client.Database(e.Database).Collection(e.Collection)
}

func (e *Connection) SendEmail(emailData pojo.EmailModel) (string, error) {

	err := sendMailWithAttachment(emailData)

	if err != nil {
		return "", err
	}

	fmt.Println("Email Sent")
	shouldReturn := e.saveMethod(emailData)
	if shouldReturn != nil {
		return "", shouldReturn
	}
	return "Email Sent Successfully", nil
}

func (*Connection) saveMethod(emailData pojo.EmailModel) error {

	var emailSave pojo.EmailPojo
	emailSave.EmailTo = emailData.EmailTo
	emailSave.EmailBCC = emailData.EmailBCC
	emailSave.EmailCC = emailData.EmailCC
	emailSave.EmailSubject = emailData.EmailSubject
	emailSave.EmailBody = emailData.EmailBody
	emailSave.Date = time.Now()

	_, err := Collection.InsertOne(ctx, emailSave)

	if err != nil {
		return errors.New("Unable To Insert New Record")
	}
	return err
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

func (e *Connection) SearchByEmailId(emailId string) (string, error) {
	var searchResult []*pojo.EmailPojo
	os.MkdirAll("data/download", os.ModePerm)
	dir := "data/download/"
	file := "searchResult" + fmt.Sprintf("%v", time.Now().Format("3_4_5_pm"))

	id, err := primitive.ObjectIDFromHex(emailId)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	filter := bson.D{}

	filter = append(filter, primitive.E{Key: "_id", Value: id})
	cur, err := Collection.Find(ctx, filter)
	if err != nil {
		fmt.Println(err)
		return "", err
	}
	for cur.Next(ctx) {
		var data pojo.EmailPojo
		err = cur.Decode(&data)
		if err != nil {
			fmt.Println(err)
			return "", err
		}
		searchResult = append(searchResult, &data)
	}

	log.Println("Pdf")
	_, errPdf := writeDataIntoPDFTable(dir, file, searchResult)
	if errPdf != nil {
		fmt.Println(errPdf)
		return "", err
	}
	// dataty, err2 := ioutil.ReadFile(dir + file + ".pdf")
	// fmt.Println("Data length", len(dataty))
	// if err2 != nil {
	// 	return "", err
	// }
	return "PDF Saved Successfully Having File name: " + file, nil
}

func writeDataIntoPDFTable(dir, file string, data []*pojo.EmailPojo) (*creator.Creator, error) {

	c := creator.New()
	c.SetPageMargins(20, 20, 20, 20)

	font, err := model.NewStandard14Font(model.HelveticaName)
	if err != nil {
		return c, err
	}

	// Bold font
	fontBold, err := model.NewStandard14Font(model.HelveticaBoldName)
	if err != nil {
		return c, err
	}

	// Generate basic usage chapter.
	if err := basicUsage(c, font, fontBold, data); err != nil {
		return c, err
	}

	err = c.WriteToFile(dir + file + ".pdf")
	if err != nil {
		return c, err
	}
	return c, nil
}

func basicUsage(c *creator.Creator, font, fontBold *model.PdfFont, data []*pojo.EmailPojo) error {
	// Create chapter.
	ch := c.NewChapter("Search Data")
	ch.SetMargins(0, 0, 10, 0)
	ch.GetHeading().SetFont(font)
	ch.GetHeading().SetFontSize(20)
	ch.GetHeading().SetColor(creator.ColorRGBFrom8bit(72, 86, 95))

	contentAlignH(c, ch, font, fontBold, data)

	// Draw chapter.
	if err := c.Draw(ch); err != nil {
		return err
	}

	return nil
}

func contentAlignH(c *creator.Creator, ch *creator.Chapter, font, fontBold *model.PdfFont, data []*pojo.EmailPojo) {

	//	normalFontColor := creator.ColorRGBFrom8bit(72, 86, 95)
	normalFontColorGreen := creator.ColorRGBFrom8bit(4, 79, 3)
	normalFontSize := 10.0
	for i := range data {
		// p := c.NewParagraph("Id" + " :     " + data[i].ID.String())
		// p.SetFont(font)
		// p.SetFontSize(normalFontSize)
		// p.SetColor(normalFontColorGreen)
		// p.SetMargins(0, 0, 10, 0)
		// ch.Add(p)
		x := c.NewParagraph("To" + " :     " + convertArrayOfStringIntoString(data[i].EmailTo))
		x.SetFont(font)
		x.SetFontSize(normalFontSize)
		x.SetColor(normalFontColorGreen)
		x.SetMargins(0, 0, 10, 0)
		ch.Add(x)
		y := c.NewParagraph("Cc" + " :     " + convertArrayOfStringIntoString(data[i].EmailCC))
		y.SetFont(font)
		y.SetFontSize(normalFontSize)
		y.SetColor(normalFontColorGreen)
		y.SetMargins(0, 0, 10, 0)
		ch.Add(y)
		z := c.NewParagraph("Bcc" + " :     " + convertArrayOfStringIntoString(data[i].EmailBCC))
		z.SetFont(font)
		z.SetFontSize(normalFontSize)
		z.SetColor(normalFontColorGreen)
		z.SetMargins(0, 0, 10, 0)
		ch.Add(z)
		b := c.NewParagraph(convertArrayOfStringIntoString(data[i].EmailSubject))
		b.SetFont(font)
		b.SetFontSize(normalFontSize)
		b.SetColor(normalFontColorGreen)
		b.SetMargins(0, 0, 10, 0)
		ch.Add(b)
		a := c.NewParagraph(data[i].EmailBody)
		a.SetFont(font)
		a.SetFontSize(normalFontSize)
		a.SetColor(normalFontColorGreen)
		a.SetMargins(0, 0, 10, 0)
		a.SetLineHeight(2)
		//	a.SetTextAlignment(creator.TextAlignmentJustify)
		ch.Add(a)
	}
}

func convertArrayOfStringIntoString(str []string) string {
	finalData := ""
	y := 0
	for x := range str {
		if y != 0 {
			finalData = finalData + ", "
		}
		finalData = finalData + str[x]
		y++
	}
	y = 0
	return finalData
}

func sendMailWithAttachment(emailData pojo.EmailModel) error {
	m := gomail.NewMessage()
	m.SetHeaders(map[string][]string{
		"From":    {m.FormatAddress("ramashankar.kumar@gridinfocom.com", "Ramashankar")},
		"To":      emailData.EmailTo,
		"Cc":      emailData.EmailCC,
		"Subject": emailData.EmailSubject,
	})
	for i := range emailData.FileLocation {
		m.Attach(emailData.FileLocation[i])
	}
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

func (e *Connection) SendEmailAttachMent(emailPojo pojo.EmailPojo, files []*multipart.FileHeader) (string, error) {

	arrayfiles, err := uploadFiles(files)
	if err != nil {
		return "", err
	}

	fmt.Println("Files:", arrayfiles)
	emailModel := setValueInEmailModel(emailPojo, arrayfiles)
	fmt.Println("EmailModel:", emailModel)
	err = sendMailWithAttachment(emailModel)
	if err != nil {
		return "", err
	}

	fmt.Println("Email Sent")
	emailPojo.Date = time.Now()
	_, err = Collection.InsertOne(ctx, emailPojo)

	if err != nil {
		return "", errors.New("Unable To Insert New Record")
	}
	return "Email Sent Successfully", nil
}

func uploadFiles(files []*multipart.FileHeader) ([]string, error) {
	var fileNames []string
	for _, fileHeader := range files {
		fileName = fileHeader.Filename
		fileNames = append(fileNames, dir+fileName)
		if fileHeader.Size > maxUploadSize {
			return fileNames, errors.New("The uploaded image is too big: %s. Please use an image less than 1MB in size: " + fileHeader.Filename)
		}

		// Open the file
		file, err := fileHeader.Open()
		if err != nil {
			return fileNames, err
		}

		defer file.Close()

		buff := make([]byte, 512)
		_, err = file.Read(buff)
		if err != nil {
			return fileNames, err
		}

		_, err = file.Seek(0, io.SeekStart)
		if err != nil {
			return fileNames, err
		}

		err = os.MkdirAll(dir, os.ModePerm)
		if err != nil {
			return fileNames, err
		}

		f, err := os.Create(dir + fileHeader.Filename)
		if err != nil {
			return fileNames, err
		}

		defer f.Close()

		_, err = io.Copy(f, file)
		if err != nil {
			return fileNames, err
		}
	}
	return fileNames, nil
}

func setValueInEmailModel(emailPojo pojo.EmailPojo, arrayFiles []string) pojo.EmailModel {
	var emailModel pojo.EmailModel
	emailModel.EmailTo = emailPojo.EmailTo
	emailModel.EmailCC = emailPojo.EmailCC
	emailModel.EmailBCC = emailPojo.EmailBCC
	emailModel.EmailSubject = emailPojo.EmailSubject
	emailModel.EmailBody = emailPojo.EmailBody
	emailModel.FileLocation = arrayFiles
	return emailModel
}
