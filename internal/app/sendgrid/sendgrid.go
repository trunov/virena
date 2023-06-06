package sendgrid

import (
	"log"
	"time"

	"github.com/sendgrid/sendgrid-go"
	"github.com/sendgrid/sendgrid-go/helpers/mail"
	"github.com/trunov/virena/internal/app/postgres"
)

func SendOrderEmail(client *sendgrid.Client, orderID string, orderData postgres.Order, createdDate time.Time) error {
	from := mail.NewEmail("Virena", "info@virena.ee")
	to := mail.NewEmail(orderData.PersonalInformation.Name, orderData.PersonalInformation.Email)
	subject := "Invoice order"

	var totalAmount float64
	for _, product := range orderData.Cart {
		totalAmount += product.Amount
	}

	templateData := map[string]interface{}{
		"orderNumber": orderID,
		"clientName":  orderData.PersonalInformation.Name,
		"orderDate":   createdDate,
		"orderItems":  orderData.Cart,
		"totalAmount": totalAmount,
	}

	personalization := mail.NewPersonalization()
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
		log.Fatalf("error sending email: %s", err)
	}

	return nil
}
