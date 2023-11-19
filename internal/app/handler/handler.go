package handler

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/sendgrid/sendgrid-go"
	"github.com/trunov/virena/internal/app/postgres"
	sg "github.com/trunov/virena/internal/app/sendgrid"
	"github.com/trunov/virena/internal/app/util"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog"
)

type Handler struct {
	dbStorage      postgres.DBStorager
	logger         zerolog.Logger
	sendGridClient *sendgrid.Client
}

func NewHandler(dbStorage postgres.DBStorager, logger zerolog.Logger, sendGridAPIKey string) *Handler {
	sendGridClient := sendgrid.NewSendClient(sendGridAPIKey)
	return &Handler{dbStorage: dbStorage, logger: logger, sendGridClient: sendGridClient}
}

func (h *Handler) GetProductResults(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "code")
	ctx := context.Background()

	country := r.Header.Get("X-Country")

	products, err := h.dbStorage.GetProductResults(ctx, productID)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		h.logger.Err(err).Msg("Get product. Something went wrong with database.")
		return
	}

	if country == "Estonia" || country == "Finland" {
		brandPercentageMap, err := h.dbStorage.GetAllBrandsPercentage(ctx)
		if err != nil {
			http.Error(w, "Something went wrong", http.StatusInternalServerError)
			h.logger.Err(err).Msg("Failed to retrieve brand percentages.")
			return
		}

		for i := range products {
			if percentage, ok := brandPercentageMap[products[i].Brand]; ok {
				products[i].Price *= (1 + percentage)
			}
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(products); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
}

func (h *Handler) SaveOrder(w http.ResponseWriter, r *http.Request) {
	var order postgres.Order
	ctx := context.Background()

	err := json.NewDecoder(r.Body).Decode(&order)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		h.logger.Err(err).Msg("Save order. Something went wrong with decoding data.")
		return
	}

	orderID := util.GenerateOrderID()
	exists, err := h.dbStorage.CheckOrderIDExists(ctx, orderID)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	for exists {
		orderID = util.GenerateOrderID()
		exists, err = h.dbStorage.CheckOrderIDExists(ctx, orderID)
		if err != nil {
			http.Error(w, "Something went wrong", http.StatusInternalServerError)
			return
		}
	}

	createdDate, err := h.dbStorage.SaveOrder(ctx, order, orderID)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	// send sendgrid email
	sg.SendOrderEmail(h.sendGridClient, orderID, order, createdDate, h.logger)

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) SendCustomerMessage(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(32 << 20)
	if err != nil {
		http.Error(w, "Error parsing form data", http.StatusInternalServerError)
		h.logger.Error().Err(err).Msg("Error parsing form data. File size is larger than 32MB.")
		return
	}

	formData := make(map[string]string)
	formData["name"] = r.FormValue("name")
	formData["email"] = r.FormValue("email")
	formData["subject"] = r.FormValue("subject")
	formData["message"] = r.FormValue("message")

	file, fileHeader, err := r.FormFile("fileAttachment")
	if err != nil && err != http.ErrMissingFile {
		http.Error(w, "Error retrieving the file", http.StatusInternalServerError)
		h.logger.Error().Err(err).Msg("Error retrieving the file")
		return
	}
	if file != nil {
		defer file.Close()
	}

	err = sg.SendCustomerMessageEmail(h.sendGridClient, formData, fileHeader, h.logger)
	if err != nil {
		http.Error(w, "Error sending email", http.StatusInternalServerError)
		h.logger.Error().Err(err).Msg("Error sending email")
		return
	}

	w.WriteHeader(http.StatusOK)
}

func (h *Handler) ProcessCSVFiles(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(128 << 20)
	if err != nil {
		http.Error(w, "Error parsing form data", http.StatusInternalServerError)
		h.logger.Error().Err(err).Msg("Error parsing form data. File size is larger than 128MB.")
		return
	}

	priceDelimiter := r.FormValue("priceDelimiter")
	priceAndCodeOrder := r.FormValue("priceAndCodeOrder")
	productDelimiter := r.FormValue("productDelimiter")
	productOrder := r.FormValue("productOrder")
	percentage := r.FormValue("percentage")

	priceFile, _, err := r.FormFile("priceFile")
	if err != nil {
		http.Error(w, "Error retrieving the price file", http.StatusInternalServerError)
		h.logger.Error().Err(err).Msg("Error retrieving the price file")
		return
	}
	defer priceFile.Close()

	productFile, _, err := r.FormFile("productFile")
	if err != nil {
		http.Error(w, "Error retrieving the product file", http.StatusInternalServerError)
		h.logger.Error().Err(err).Msg("Error retrieving the product file")
		return
	}
	defer productFile.Close()

	priceReader := csv.NewReader(priceFile)
	if priceDelimiter == ";" {
		priceReader.Comma = ';'
	} else {
		priceReader.Comma = ','
	}

	priceAndCodeOrderSplit := strings.Split(priceAndCodeOrder, ",")
	// trim for priceAndCodeOrder
	productOrderIndex, err := strconv.Atoi(productOrder)
	if err != nil || len(priceAndCodeOrderSplit) != 2 {
		http.Error(w, "Invalid order values", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid order values")
		return
	}
	priceIndex, err := strconv.Atoi(priceAndCodeOrderSplit[0])
	if err != nil {
		http.Error(w, "Invalid index values in order", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid index values in order")
		return
	}

	codeIndex, err := strconv.Atoi(priceAndCodeOrderSplit[1])
	if err != nil {
		http.Error(w, "Invalid index values in order", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid index values in order")
		return
	}

	percentageNum, err := strconv.Atoi(percentage)
	if err != nil {
		http.Error(w, "Invalid index values in order", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid index values in order")
		return
	}

	// Adjust indices (assuming they start from 1 in the input)
	priceIndex--
	codeIndex--
	productOrderIndex--

	// Creating a map for prices
	pricesMap := make(map[string]string)
	for {
		record, err := priceReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "Error reading the price file", http.StatusInternalServerError)
			h.logger.Error().Err(err).Msg("Error reading the price file")
			return
		}

		if len(record) > codeIndex && len(record) > priceIndex {
			partCode := record[codeIndex]
			partPrice := record[priceIndex]
			pricesMap[partCode] = partPrice
		}
	}

	productReader := csv.NewReader(productFile)
	if productDelimiter == ";" {
		productReader.Comma = ';'
	} else {
		productReader.Comma = ','
	}

	productRecords, err := productReader.ReadAll()
	if err != nil {
		http.Error(w, "Error reading the product file", http.StatusInternalServerError)
		h.logger.Error().Err(err).Msg("Error reading the product file")
		return
	}

	for i, record := range productRecords {
		if i == 0 {
			if priceIndex == len(record)-1 {
				record = append(record, "new price")
			} else {
				record = append(record[:priceIndex+1], append([]string{"new price"}, record[priceIndex+1:]...)...)
			}
			productRecords[i] = record
			continue
		}

		var newPriceStr string
		if originalPrice, ok := pricesMap[record[productOrderIndex]]; ok {
			originalPrice = strings.ReplaceAll(originalPrice, " ", "")
			originalPrice = strings.ReplaceAll(originalPrice, ",", ".")

			if price, err := strconv.ParseFloat(originalPrice, 64); err == nil {
				newPrice := price * (1 + float64(percentageNum)/100)
				newPriceStr = fmt.Sprintf("%.2f", newPrice)
			}
		}

		if newPriceStr == "" {
			newPriceStr = "N/A"
		}

		if priceIndex == len(record)-1 {
			record = append(record, newPriceStr)
		} else {
			record = append(record[:priceIndex+1], append([]string{newPriceStr}, record[priceIndex+1:]...)...)
		}
		productRecords[i] = record
	}

	w.Header().Set("Content-Disposition", "attachment; filename=updated_products.csv")
	w.Header().Set("Content-Type", "text/csv")
	csvWriter := csv.NewWriter(w)
	err = csvWriter.WriteAll(productRecords)
	if err != nil {
		http.Error(w, "Error writing to output file", http.StatusInternalServerError)
		h.logger.Error().Err(err).Msg("Error writing to output file")
		return
	}
}

func (h *Handler) PingDB(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	if err := h.dbStorage.Ping(ctx); err != nil {
		h.logger.Err(err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	h.logger.Info().Msg("Ping to db was successful")
	w.WriteHeader(http.StatusOK)
}

func NewRouter(h *Handler) chi.Router {
	r := chi.NewRouter()

	r.Use(cors.Handler(cors.Options{
		AllowedOrigins:   []string{"https://www.virena.ee", "http://localhost:3000"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token", "X-Country"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of the major browsers
	}))

	r.Get("/ping", h.PingDB)
	r.Route("/api", func(r chi.Router) {
		r.Get("/product/{code}/results", h.GetProductResults)
		r.Post("/order", h.SaveOrder)
		r.Post("/contact", h.SendCustomerMessage)
		r.Post("/handle-csv", h.ProcessCSVFiles)
	})

	return r
}
