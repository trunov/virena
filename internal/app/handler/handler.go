package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"net/http"

	"github.com/trunov/virena/internal/app/postgres"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/cors"
	"github.com/rs/zerolog"
)

type Handler struct {
	dbStorage postgres.DBStorager
	logger    zerolog.Logger
}

func NewHandler(dbStorage postgres.DBStorager, logger zerolog.Logger) *Handler {
	return &Handler{dbStorage: dbStorage, logger: logger}
}

func (h *Handler) GetProduct(w http.ResponseWriter, r *http.Request) {
	productID := chi.URLParam(r, "code")
	ctx := context.Background()

	product, err := h.dbStorage.GetProduct(ctx, productID)
	if err != nil {
		if err == sql.ErrNoRows {
			w.WriteHeader(http.StatusNoContent)
			return
		}
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		h.logger.Err(err).Msg("Get product. Something went wrong with database.")
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(product); err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
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
	r.Get("/api/product/{code}", h.GetProduct)

	return r
}
