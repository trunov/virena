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
	"github.com/trunov/virena/internal/app/services"
	"github.com/trunov/virena/internal/app/util"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog"
)

type PriceDealerInfo struct {
	Price       string
	Dealer      string
	Description string
}

type Handler struct {
	dbStorage      postgres.DBStorager
	logger         zerolog.Logger
	service        services.FileService
	sendGridClient *sendgrid.Client
}

func NewHandler(dbStorage postgres.DBStorager, service services.FileService, logger zerolog.Logger, sendGridAPIKey string) *Handler {
	sendGridClient := sendgrid.NewSendClient(sendGridAPIKey)
	return &Handler{dbStorage: dbStorage, service: service, logger: logger, sendGridClient: sendGridClient}
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

func (h *Handler) ProcessPriceCSVFiles(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(128 << 20)
	if err != nil {
		http.Error(w, "Error parsing form data", http.StatusInternalServerError)
		h.logger.Error().Err(err).Msg("Error parsing form data. File size is larger than 128MB.")
		return
	}

	defer func() {
		if r := recover(); r != nil {
			errMsg := fmt.Sprintf("Panic occurred: %v", r)
			h.logger.Error().Msg(errMsg)
			http.Error(w, errMsg, http.StatusInternalServerError)
		}
	}()

	priceDelimiter := r.FormValue("priceDelimiter")
	priceCodeDescriptionOrder := r.FormValue("priceCodeDescriptionOrder")
	productDelimiter := r.FormValue("productDelimiter")
	productOrder := r.FormValue("productOrder")
	percentage := r.FormValue("percentage")
	// let's add column identifier which will be saved by name dealer if number is presented
	dealerColumnStr := r.FormValue("dealerColumn")
	dealerColumn := -1

	if dealerColumnStr != "" {
		var err error
		dealerColumn, err = strconv.Atoi(dealerColumnStr)
		if err != nil {
			http.Error(w, "Invalid dealer column value", http.StatusBadRequest)
			h.logger.Error().Err(err).Msg("Invalid dealer column value")
			return
		}
		dealerColumn-- // Adjust for 0-indexing
	}

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
	priceReader.FieldsPerRecord = -1

	priceCodeDescriptionOrderSplit := strings.Split(priceCodeDescriptionOrder, ",")
	// trim for priceAndCodeOrder
	productOrderIndex, err := strconv.Atoi(productOrder)
	if err != nil || len(priceCodeDescriptionOrderSplit) != 3 {
		http.Error(w, "Invalid order values", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid order values")
		return
	}
	priceIndex, err := strconv.Atoi(priceCodeDescriptionOrderSplit[0])
	if err != nil {
		http.Error(w, "Invalid index values in order", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid index values in order")
		return
	}

	codeIndex, err := strconv.Atoi(priceCodeDescriptionOrderSplit[1])
	if err != nil {
		http.Error(w, "Invalid index values in order", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid index values in order")
		return
	}

	descriptionIndex, err := strconv.Atoi(priceCodeDescriptionOrderSplit[2])
	if err != nil {
		http.Error(w, "Invalid index values in order", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid index values in order")
		return
	}

	percentageNum, err := strconv.ParseFloat(percentage, 64)
	if err != nil {
		http.Error(w, "Invalid index values in order", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid index values in order")
		return
	}

	// Adjust indices (assuming they start from 1 in the input)
	priceIndex--
	codeIndex--
	productOrderIndex--
	descriptionIndex--

	// Creating a map for prices
	pricesMap := make(map[string]PriceDealerInfo)
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

			// we don't take this if client provides 0 as argument
			var description string
			if descriptionIndex >= 0 {
				description = record[descriptionIndex]
			} else {
				description = ""
			}

			var dealerInfo string
			if dealerColumn >= 0 && len(record) > dealerColumn {
				dealerInfo = record[dealerColumn]
			}
			pricesMap[partCode] = PriceDealerInfo{Price: partPrice, Dealer: dealerInfo, Description: description}
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

			if dealerColumn >= 0 {
				record = append(record, "dealer")
			}

			if descriptionIndex >= 0 {
				record = append(record, "replacement code")
			}

			productRecords[i] = record
			continue
		}

		productCode := record[productOrderIndex]

		info, ok := pricesMap[productCode]
		if !ok {
			if productCode[0] == '0' {
				unprefixedCode := productCode[1:]
				info, ok = pricesMap[unprefixedCode]
			} else {
				prefixedCode := "0" + productCode
				info, ok = pricesMap[prefixedCode]
			}
		}

		var newPriceStr string
		if ok {
			cleanedPrice := strings.ReplaceAll(info.Price, " ", "")
			cleanedPrice = strings.ReplaceAll(cleanedPrice, ",", ".")

			if price, err := strconv.ParseFloat(cleanedPrice, 64); err == nil {
				newPrice := price * (1 + percentageNum/100)
				if newPrice > 10 {
					newPriceStr = fmt.Sprintf("%.2f", newPrice)
				} else {
					newPriceStr = fmt.Sprintf("%.3f", newPrice)
				}
			} else {
				newPriceStr = "N/A"
			}
		} else {
			newPriceStr = "N/A"
		}

		if priceIndex == len(record)-1 {
			record = append(record, newPriceStr)
		} else {
			record = append(record[:priceIndex+1], append([]string{newPriceStr}, record[priceIndex+1:]...)...)
		}

		if dealerColumn >= 0 {
			dealerInfo := info.Dealer
			record = append(record, dealerInfo)
		}

		if descriptionIndex >= 0 {
			potentiallyReplacementCode := info.Description
			info, ok = pricesMap[potentiallyReplacementCode]
			if ok {
				record = append(record, potentiallyReplacementCode)
			} else {
				record = append(record, "")
			}
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

func (h *Handler) ProcessDealerCSVFiles(w http.ResponseWriter, r *http.Request) {
	ctx := context.Background()

	err := r.ParseMultipartForm(128 << 20)
	if err != nil {
		http.Error(w, "Error parsing form data", http.StatusInternalServerError)
		h.logger.Error().Err(err).Msg("Error parsing form data. File size is larger than 128MB.")
		return
	}

	dealerOneDelimiter := r.FormValue("dealerOneDelimiter")
	dealerOneOrderPriceWeightAndDescription := r.FormValue("dealerOneOrderPriceWeightAndDescription")

	dealerTwoDelimiter := r.FormValue("dealerTwoDelimiter")
	dealerTwoOrderPriceAndDescription := r.FormValue("dealerTwoOrderPriceAndDescription")

	dealerColumnStr := r.FormValue("dealerColumn")
	dealerNumberStr := r.FormValue("dealerNumber")

	dealerColumn := -1
	var dealerNumber int

	withAdditionalData := r.FormValue("withAdditionalData")

	if dealerColumnStr != "" {
		var err error
		dealerColumn, err = strconv.Atoi(dealerColumnStr)
		if err != nil {
			http.Error(w, "Invalid dealer column value", http.StatusBadRequest)
			h.logger.Error().Err(err).Msg("Invalid dealer column value")
			return
		}
		dealerColumn-- // Adjust for 0-indexing

		dealerNumber, err = strconv.Atoi(dealerNumberStr)
		if err != nil {
			http.Error(w, "Invalid dealer number value", http.StatusBadRequest)
			h.logger.Error().Err(err).Msg("Invalid dealer number value")
			return
		}
	}

	dealerOne, _, err := r.FormFile("dealerOne")
	if err != nil {
		http.Error(w, "Error retrieving the dealerOne file", http.StatusInternalServerError)
		h.logger.Error().Err(err).Msg("Error retrieving the dealerOne file")
		return
	}
	defer dealerOne.Close()

	dealerTwo, _, err := r.FormFile("dealerTwo")
	if err != nil {
		http.Error(w, "Error retrieving the dealerTwo file", http.StatusInternalServerError)
		h.logger.Error().Err(err).Msg("Error retrieving the dealerTwo file")
		return
	}
	defer dealerTwo.Close()

	dealerOneOrderPriceWeightAndDescriptionSplit := strings.Split(dealerOneOrderPriceWeightAndDescription, ",")
	if len(dealerOneOrderPriceWeightAndDescriptionSplit) != 4 {
		http.Error(w, "Invalid order values", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid dealer one values")
		return
	}
	dealerOnePriceIndex, err := strconv.Atoi(dealerOneOrderPriceWeightAndDescriptionSplit[0])
	if err != nil {
		http.Error(w, "Invalid index values in dealer one", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid index values in dealer one")
		return
	}

	dealerOneCodeIndex, err := strconv.Atoi(dealerOneOrderPriceWeightAndDescriptionSplit[1])
	if err != nil {
		http.Error(w, "Invalid index values in order", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid index values in dealer one")
		return
	}

	dealerOneDescriptionIndex, err := strconv.Atoi(dealerOneOrderPriceWeightAndDescriptionSplit[2])
	if err != nil {
		http.Error(w, "Invalid index values in order", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid index values in dealer one")
		return
	}

	dealerOneWeightIndex, err := strconv.Atoi(dealerOneOrderPriceWeightAndDescriptionSplit[3])
	if err != nil {
		http.Error(w, "Invalid index values in order", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid index values in dealer one")
		return
	}

	// Adjust indices (assuming they start from 1 in the input)
	dealerOnePriceIndex--
	dealerOneCodeIndex--
	dealerOneDescriptionIndex--
	dealerOneWeightIndex--

	dealerTwoReader := csv.NewReader(dealerTwo)
	if dealerTwoDelimiter == ";" {
		dealerTwoReader.Comma = ';'
	} else {
		dealerTwoReader.Comma = ','
	}

	dealerTwoOrderPriceAndDescriptionSplit := strings.Split(dealerTwoOrderPriceAndDescription, ",")
	if len(dealerTwoOrderPriceAndDescriptionSplit) != 3 {
		http.Error(w, "Invalid order values", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid dealer one values")
		return
	}
	dealerTwoPriceIndex, err := strconv.Atoi(dealerTwoOrderPriceAndDescriptionSplit[0])
	if err != nil {
		http.Error(w, "Invalid index values in dealer one", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid index values in dealer one")
		return
	}

	dealerTwoCodeIndex, err := strconv.Atoi(dealerTwoOrderPriceAndDescriptionSplit[1])
	if err != nil {
		http.Error(w, "Invalid index values in order", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid index values in dealer one")
		return
	}

	dealerTwoDescriptionIndex, err := strconv.Atoi(dealerTwoOrderPriceAndDescriptionSplit[2])
	if err != nil {
		http.Error(w, "Invalid index values in order", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid index values in dealer one")
		return
	}

	dealerTwoPriceIndex--
	dealerTwoCodeIndex--
	dealerTwoDescriptionIndex--

	fmt.Println(dealerOnePriceIndex)
	fmt.Println(dealerOneCodeIndex)
	fmt.Println(dealerOneDescriptionIndex)
	fmt.Println(dealerOneWeightIndex)

	d1, err := h.service.ReadFile(ctx, dealerOne, rune(dealerOneDelimiter[0]), dealerOnePriceIndex, dealerOneCodeIndex, dealerColumn, dealerOneDescriptionIndex, dealerOneWeightIndex)
	if err != nil {
		http.Error(w, "Could not read dealer one file", http.StatusInternalServerError)
		h.logger.Error().Err(err).Msg("Could not read dealer one file")
		return
	}

	d2, err := h.service.ReadFileToMap(ctx, dealerTwo, rune(dealerTwoDelimiter[0]), dealerTwoPriceIndex, dealerTwoCodeIndex, dealerTwoDescriptionIndex)
	if err != nil {
		http.Error(w, "Could not read dealer two file", http.StatusInternalServerError)
		h.logger.Error().Err(err).Msg("Could not read dealer two file")
		return
	}

	res, err := h.service.CompareAndProcessFiles(ctx, d1, d2, dealerColumn, dealerNumber, withAdditionalData)
	if err != nil {
		http.Error(w, "Failed during comparison of dealers", http.StatusInternalServerError)
		h.logger.Error().Err(err).Msg("Failed during comparison of dealers")
		return
	}

	w.Header().Set("Content-Disposition", "attachment; filename=updated_products.csv")
	w.Header().Set("Content-Type", "text/csv")
	csvWriter := csv.NewWriter(w)
	err = csvWriter.WriteAll(res)
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
		r.Post("/handle-price-csv", h.ProcessPriceCSVFiles)
		r.Post("/handle-dealer-csv", h.ProcessDealerCSVFiles)
	})

	return r
}
