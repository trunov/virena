package handler

import (
	"context"
	"encoding/json"
	"net/http"

	"github.com/sendgrid/sendgrid-go"
	"github.com/trunov/virena/internal/app/postgres"
	sg "github.com/trunov/virena/internal/app/sendgrid"

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

	// would be nice to validate data

	orderID, createdDate, err := h.dbStorage.SaveOrder(ctx, order)
	if err != nil {
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	// send sendgrid email
	sg.SendOrderEmail(h.sendGridClient, orderID, order, createdDate, h.logger)

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
		// AllowedOrigins:   []string{"https://foo.com"}, // Use this to allow specific origin hosts
		AllowedOrigins: []string{"https://*", "http://*"},
		// AllowOriginFunc:  func(r *http.Request, origin string) bool { return true },
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Accept", "Authorization", "Content-Type", "X-CSRF-Token"},
		ExposedHeaders:   []string{"Link"},
		AllowCredentials: false,
		MaxAge:           300, // Maximum value not ignored by any of major browsers
	}))

	r.Get("/ping", h.PingDB)
	r.Get("/api/product/{code}/results", h.GetProductResults)
	r.Post("/api/order", h.SaveOrder)

	return r
}
