package sendgrid

import (
	"fmt"
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

	var summ float64
	for _, product := range orderData.Cart {
		summ += product.Amount
	}

	formattedSumm := fmt.Sprintf("%.2f", summ)
	kabemaks := fmt.Sprintf("%.2f", summ * 0.2)
	totalAmount := fmt.Sprintf("%.2f", summ * 1.2)

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
		"orderDate":   createdDate,
		"orderItems":  orderItems,
		"summ": formattedSumm,
		"kabemaks": kabemaks,
		"totalAmount": totalAmount,
	}

	personalization := mail.NewPersonalization()
	personalization.AddTos(to)
	personalization.AddCCs(cc)
	for key, value := range templateData {
		personalization.SetDynamicTemplateData(key, value)
	}

	message := mail.NewSingleEmail(from, subject, to, "", "")
	message.SetTemplateID("d-6b824c66024e48acb1f0aa1fff9fd4e0")
	message.AddPersonalizations(personalization)

	_, err := client.Send(message)
	if err != nil {
		logger.Error().Err(err)
	}

	return nil
}
