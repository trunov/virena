package handler

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sendgrid/sendgrid-go"
	"github.com/trunov/virena/internal/app/postgres"
	sg "github.com/trunov/virena/internal/app/sendgrid"
	"github.com/trunov/virena/internal/app/services"
	"github.com/trunov/virena/internal/app/util"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog"
)

type CodeInfo struct {
	Price             string
	Dealer            string
	WorstPrice        string
	WorstDealerNumber string
	PriceRatio        string
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

	var fileHeaders []*multipart.FileHeader

	multipartForm := r.MultipartForm
	for key := range multipartForm.File {
		if strings.HasPrefix(key, "fileAttachment") {
			fileHeaders = append(fileHeaders, multipartForm.File[key]...)
		}
	}

	err = sg.SendCustomerMessageEmail(h.sendGridClient, formData, fileHeaders, h.logger)
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

	withAdditionalData := r.FormValue("withAdditionalData")
	priceDelimiter := r.FormValue("priceDelimiter")
	priceAndCodeOrder := r.FormValue("priceAndCodeOrder")
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

	// Creating a map for prices
	pricesMap := make(map[string]CodeInfo)
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

		recordLength := len(record)

		if recordLength > codeIndex && recordLength > priceIndex {
			partCode := record[codeIndex]
			partPrice := record[priceIndex]

			var dealerInfo string
			if dealerColumn >= 0 && len(record) > dealerColumn {
				dealerInfo = record[dealerColumn]
			}

			if withAdditionalData == "" {
				pricesMap[partCode] = CodeInfo{Price: partPrice, Dealer: dealerInfo}
			} else {
				pricesMap[partCode] = CodeInfo{
					Price:             partPrice,
					Dealer:            dealerInfo,
					WorstPrice:        record[recordLength-3],
					WorstDealerNumber: record[recordLength-2],
					PriceRatio:        record[recordLength-1],
				}
			}
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

			if withAdditionalData != "" {
				record = append(record, "Worst Price", "Worst Dealer Number", "Price Ratio")
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

		if withAdditionalData != "" {
			record = append(record, info.WorstPrice, info.WorstDealerNumber, info.PriceRatio)
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
	dealerOnePriceAndCodeOrder := r.FormValue("dealerOnePriceAndCodeOrder")

	dealerTwoDelimiter := r.FormValue("dealerTwoDelimiter")
	dealerTwoPriceAndCodeOrder := r.FormValue("dealerTwoPriceAndCodeOrder")

	dealerColumnStr := r.FormValue("dealerColumn")
	secondDealerNumberStr := r.FormValue("secondDealerNumber")

	offsetPercentageStr := r.FormValue("offsetPercentage")

	firstDealerNumber := r.FormValue("firstDealerNumber")

	dealerColumn := -1
	var secondDealerNumber, offsetPercentage int

	if offsetPercentageStr != "" {
		var err error
		offsetPercentage, err = strconv.Atoi(offsetPercentageStr)
		if err != nil {
			http.Error(w, "Invalid offset percentage value", http.StatusBadRequest)
			h.logger.Error().Err(err).Msg("Invalid offset percentage value")
			return
		}
	}

	if dealerColumnStr != "" {
		var err error
		dealerColumn, err = strconv.Atoi(dealerColumnStr)
		if err != nil {
			http.Error(w, "Invalid dealer column value", http.StatusBadRequest)
			h.logger.Error().Err(err).Msg("Invalid dealer column value")
			return
		}
		dealerColumn-- // Adjust for 0-indexing

		secondDealerNumber, err = strconv.Atoi(secondDealerNumberStr)
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

	dealerOnePriceAndCodeOrderSplit := strings.Split(dealerOnePriceAndCodeOrder, ",")
	if len(dealerOnePriceAndCodeOrderSplit) != 2 {
		http.Error(w, "Invalid order values", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid dealer one values")
		return
	}

	dealerOnePriceIndex, err := strconv.Atoi(dealerOnePriceAndCodeOrderSplit[0])
	if err != nil {
		http.Error(w, "Invalid index values in dealer one", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid index values in dealer one")
		return
	}

	dealerOneCodeIndex, err := strconv.Atoi(dealerOnePriceAndCodeOrderSplit[1])
	if err != nil {
		http.Error(w, "Invalid index values in order", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid index values in dealer one")
		return
	}

	// Adjust indices (assuming they start from 1 in the input)
	dealerOnePriceIndex--
	dealerOneCodeIndex--

	dealerTwoReader := csv.NewReader(dealerTwo)
	if dealerTwoDelimiter == ";" {
		dealerTwoReader.Comma = ';'
	} else {
		dealerTwoReader.Comma = ','
	}

	dealerTwoPriceAndCodeOrderSplit := strings.Split(dealerTwoPriceAndCodeOrder, ",")
	if len(dealerTwoPriceAndCodeOrderSplit) != 2 {
		http.Error(w, "Invalid order values", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid dealer one values")
		return
	}
	dealerTwoPriceIndex, err := strconv.Atoi(dealerTwoPriceAndCodeOrderSplit[0])
	if err != nil {
		http.Error(w, "Invalid index values in dealer one", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid index values in dealer one")
		return
	}

	dealerTwoCodeIndex, err := strconv.Atoi(dealerTwoPriceAndCodeOrderSplit[1])
	if err != nil {
		http.Error(w, "Invalid index values in order", http.StatusBadRequest)
		h.logger.Error().Err(err).Msg("Invalid index values in dealer one")
		return
	}

	dealerTwoPriceIndex--
	dealerTwoCodeIndex--

	d1, err := h.service.ReadFile(ctx, dealerOne, rune(dealerOneDelimiter[0]), dealerOnePriceIndex, dealerOneCodeIndex, dealerColumn)
	if err != nil {
		http.Error(w, "Could not read dealer one file", http.StatusInternalServerError)
		h.logger.Error().Err(err).Msg("Could not read dealer one file")
		return
	}

	d2, err := h.service.ReadFileToMap(ctx, dealerTwo, rune(dealerTwoDelimiter[0]), dealerTwoPriceIndex, dealerTwoCodeIndex)
	if err != nil {
		http.Error(w, "Could not read dealer two file", http.StatusInternalServerError)
		h.logger.Error().Err(err).Msg("Could not read dealer two file")
		return
	}

	res, err := h.service.CompareAndProcessFiles(ctx, d1, d2, dealerColumn, secondDealerNumber, offsetPercentage, firstDealerNumber)
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

func (h *Handler) AttachExtraField(w http.ResponseWriter, r *http.Request) {
	err := r.ParseMultipartForm(128 << 20)
	if err != nil {
		http.Error(w, "Error parsing form data", http.StatusInternalServerError)
		h.logger.Error().Err(err).Msg("Error parsing form data. File size is larger than 128MB.")
		return
	}

	dealerOneDelimiter := rune(r.FormValue("dealerOneDelimiter")[0])
	firstDealerCodeOrderStr := r.FormValue("firstDealerCodeOrder")
	firstDealerCodeOrder, err := strconv.Atoi(firstDealerCodeOrderStr)
	if err != nil || firstDealerCodeOrder <= 0 {
		http.Error(w, "Invalid value for firstDealerCodeOrder", http.StatusBadRequest)
		h.logger.Error().Msgf("Invalid value for firstDealerCodeOrder: %s", firstDealerCodeOrderStr)
		return
	}

	dealerTwoDelimiter := rune(r.FormValue("dealerTwoDelimiter")[0])
	secondDealerCodeOrderStr := r.FormValue("secondDealerCodeOrder")
	secondDealerCodeOrder, err := strconv.Atoi(secondDealerCodeOrderStr)
	if err != nil || secondDealerCodeOrder <= 0 {
		http.Error(w, "Invalid value for secondDealerCodeOrder", http.StatusBadRequest)
		h.logger.Error().Msgf("Invalid value for secondDealerCodeOrder: %s", secondDealerCodeOrderStr)
		return
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

	extraField := r.FormValue("extraField")
	extraFieldIndex, err := strconv.Atoi(extraField)
	if err != nil || extraFieldIndex <= 0 {
		http.Error(w, "Invalid value for extraField", http.StatusBadRequest)
		h.logger.Error().Msgf("Invalid value for extraField: %s", extraField)
		return
	}

	dealerTwoReader := csv.NewReader(dealerTwo)
	dealerTwoReader.Comma = dealerTwoDelimiter

	dealerOneReader := csv.NewReader(dealerOne)
	dealerOneReader.Comma = dealerOneDelimiter

	dealerOneData := make(map[string]string)
	for {
		record, err := dealerOneReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "Error reading dealerTwo file", http.StatusInternalServerError)
			h.logger.Error().Err(err).Msg("Error reading dealerTwo file")
			return
		}
		if len(record) > secondDealerCodeOrder-1 && len(record) > extraFieldIndex-1 {
			dealerOneData[record[firstDealerCodeOrder-1]] = record[extraFieldIndex-1]
		}
	}

	var result strings.Builder
	writer := csv.NewWriter(&result)
	writer.Comma = dealerOneDelimiter

	for {
		record, err := dealerTwoReader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			http.Error(w, "Error processing dealerOne file", http.StatusInternalServerError)
			h.logger.Error().Err(err).Msg("Error processing dealerOne file")
			return
		}

		if len(record) > secondDealerCodeOrder-1 {
			code := record[secondDealerCodeOrder-1]
			if extraValue, exists := dealerOneData[code]; exists {
				record = append(record, extraValue)
			} else {
				record = append(record, "")
			}
		}
		writer.Write(record)
	}
	writer.Flush()

	w.Header().Set("Content-Disposition", "attachment; filename=updated_dealer_one.csv")
	w.Header().Set("Content-Type", "text/csv")
	w.WriteHeader(http.StatusOK)
	_, err = w.Write([]byte(result.String()))
	if err != nil {
		h.logger.Error().Err(err).Msg("Error writing response")
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

	r.Handle("/virena-metrics", promhttp.Handler())

	r.Get("/ping", h.PingDB)
	r.Route("/api", func(r chi.Router) {
		r.Get("/product/{code}/results", h.GetProductResults)
		r.Post("/order", h.SaveOrder)
		r.Post("/contact", h.SendCustomerMessage)
		r.Post("/handle-price-csv", h.ProcessPriceCSVFiles)
		r.Post("/handle-dealer-csv", h.ProcessDealerCSVFiles)
		r.Post("/attach-extra-column", h.AttachExtraField)
	})

	return r
}
