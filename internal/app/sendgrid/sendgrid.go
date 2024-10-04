package sendgrid

import (
	"encoding/base64"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/trunov/virena/internal/app/postgres"
	"github.com/trunov/virena/internal/app/util"
)

func SendOrderEmail(client *sendgrid.Client, orderID int, orderData postgres.Order, createdDate time.Time, logger zerolog.Logger) error {
	from := mail.NewEmail("Virena", "info@virena.ee")
	to := mail.NewEmail(orderData.PersonalInformation.Name, orderData.PersonalInformation.Email)
	cc := mail.NewEmail("Virena", "info@virena.ee")

	subject := "Invoice order"

	var summ float64
	for _, product := range orderData.Cart {
		summ += product.Amount
	}

	formattedSumm := fmt.Sprintf("%.2f", summ)
	kabemaks := fmt.Sprintf("%.2f", summ*0.2)
	totalAmount := fmt.Sprintf("%.2f", summ*1.2)

	var orderItems []map[string]interface{}
	for _, product := range orderData.Cart {
		price := fmt.Sprintf("%.2f", product.Price)
		amount := fmt.Sprintf("%.2f", product.Amount)

		item := map[string]interface{}{
			"partCode":    product.PartCode,
			"price":       price,
			"quantity":    product.Quantity,
			"amount":      amount,
			"description": product.Description,
		}
		orderItems = append(orderItems, item)
	}

	templateData := map[string]interface{}{
		"orderNumber": orderID,
		"clientName":  orderData.PersonalInformation.Name,
		"orderDate":   util.ConvertToGMTPlus3(createdDate),
		"orderItems":  orderItems,
		"summ":        formattedSumm,
		"kabemaks":    kabemaks,
		"totalAmount": totalAmount,
	}

	message := mail.NewV3Mail()
	message.SetFrom(from)
	message.Subject = subject

	personalization := mail.NewPersonalization()
	personalization.AddTos(to)
	personalization.AddCCs(cc)

	for key, value := range templateData {
		personalization.SetDynamicTemplateData(key, value)
	}

	message.AddPersonalizations(personalization)
	message.SetTemplateID("d-6b824c66024e48acb1f0aa1fff9fd4e0")

	_, err := client.Send(message)
	if err != nil {
		logger.Error().Err(err)
	}

	return nil
}

func SendCustomerMessageEmail(client *sendgrid.Client, formData map[string]string, fileHeaders []*multipart.FileHeader, logger zerolog.Logger) error {
	from := mail.NewEmail("Virena", "info@virena.ee")
	to := mail.NewEmail("Virena", "info@virena.ee")

	subject := "Customer Request Message"
	content := strings.Builder{}

	content.WriteString(fmt.Sprintf("Name: %s\n", formData["name"]))
	content.WriteString(fmt.Sprintf("Email: %s\n", formData["email"]))
	content.WriteString(fmt.Sprintf("Subject: %s\n", formData["subject"]))
	content.WriteString(fmt.Sprintf("Message: %s\n", formData["message"]))

	message := mail.NewV3MailInit(from, subject, to, mail.NewContent("text/plain", content.String()))

	for _, fileHeader := range fileHeaders {
		if fileHeader != nil {
			file, err := fileHeader.Open()
			if err != nil {
				logger.Error().Err(err).Msg("Failed to open file attachment")
				return err
			}
			defer file.Close()

			fileContent, err := io.ReadAll(file)
			if err != nil {
				logger.Error().Err(err).Msg("Failed to read file attachment")
				return err
			}

			encodedContent := base64.StdEncoding.EncodeToString(fileContent)
			attachment := mail.NewAttachment()
			attachment.SetContent(encodedContent)
			attachment.SetType(http.DetectContentType(fileContent))
			attachment.SetFilename(fileHeader.Filename)
			attachment.SetDisposition("attachment")
			message.AddAttachment(attachment)
		}
	}

	response, err := client.Send(message)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to send customer message email")
		return err
	}

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		err := fmt.Errorf("received non-successful response from SendGrid: %d", response.StatusCode)
		logger.Error().Err(err).Msg("Failed to send customer message email")
		return err
	}

	return nil
}
