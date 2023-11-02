package handler

import (
	"context"
	"encoding/json"
	"net/http"

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
		AllowedOrigins:   []string{"https://www.virena.ee"},
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
	})

	return r
}
