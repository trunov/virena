package sendgrid

import (
	"time"

	"github.com/rs/zerolog"
	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/trunov/virena/internal/app/postgres"
)

func SendOrderEmail(client *sendgrid.Client, orderID string, orderData postgres.Order, createdDate time.Time, logger zerolog.Logger) error {
	from := mail.NewEmail("Virena", "info@virena.ee")
	to := mail.NewEmail(orderData.PersonalInformation.Name, orderData.PersonalInformation.Email)
	cc := mail.NewEmail("Virena", "info@virena.ee")
	subject := "Invoice order"

	var totalAmount float64
	for _, product := range orderData.Cart {
		totalAmount += product.Amount
	}

	var orderItems []map[string]interface{}
	for _, product := range orderData.Cart {
		item := map[string]interface{}{
			"partCode":    product.PartCode,
			"price":       product.Price,
			"quantity":    product.Quantity,
			"amount":      product.Amount,
			"description": product.Description,
		}
		orderItems = append(orderItems, item)
	}

	templateData := map[string]interface{}{
		"orderNumber": orderID,
		"clientName":  orderData.PersonalInformation.Name,
		"orderDate":   createdDate,
		"orderItems":  orderItems,
		"totalAmount": totalAmount,
	}

	personalization := mail.NewPersonalization()
	personalization.AddCCs(cc)
	personalization.AddTos(to)
	for key, value := range templateData {
		personalization.SetDynamicTemplateData(key, value)
	}

	// Create a SendGrid dynamic template email
	message := mail.NewSingleEmail(from, subject, to, "", "")
	message.SetTemplateID("d-6b824c66024e48acb1f0aa1fff9fd4e0")
	message.AddPersonalizations(personalization)

	// Send the email
	_, err := client.Send(message)
	if err != nil {
		logger.Error().Err(err)
	}

	return nil
}
