package handlers

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/ctxval"
	"github.com/fredsaggio/url-shortener/internal/problem"
	"github.com/fredsaggio/url-shortener/internal/services"
)

const defaultClickPeriod = 30

type ClickStatsService interface {
	DailyClicks(ctx context.Context, userID uuid.UUID, shortCode string, days int) (services.ClickStats, error)
}

type ClickStatsHandler struct {
	serv ClickStatsService
}

func NewClickStatsHandler(serv ClickStatsService) *ClickStatsHandler {
	return &ClickStatsHandler{serv: serv}
}

type dayClicksResponse struct {
	Date string `json:"date"`
	Link int64  `json:"link"`
	QR   int64  `json:"qr"`
}

type clickTotalsResponse struct {
	Link int64 `json:"link"`
	QR   int64 `json:"qr"`
}

type clickStatsResponse struct {
	Days   []dayClicksResponse `json:"days"`
	Totals clickTotalsResponse `json:"totals"`
}

// DailyClicks answers GET /urls/{code}/clicks?days=7|30|90 for the owner.
func (h *ClickStatsHandler) DailyClicks(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	userID, ok := ctxval.UserIDFromContext(ctx)
	if !ok {
		problem.Write(w, r, http.StatusUnauthorized, "authentication_required", "autenticação necessária")
		return
	}

	days := defaultClickPeriod
	if raw := r.URL.Query().Get("days"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil {
			problem.Write(w, r, http.StatusBadRequest, "invalid_click_period", "período inválido; use 7, 30 ou 90 dias")
			return
		}
		days = parsed
	}

	stats, err := h.serv.DailyClicks(ctx, userID, r.PathValue("code"), days)
	if err != nil {
		switch {
		case errors.Is(err, services.ErrInvalidClickPeriod):
			problem.Write(w, r, http.StatusBadRequest, "invalid_click_period", "período inválido; use 7, 30 ou 90 dias")
		case errors.Is(err, services.ErrUnauthenticated):
			problem.Write(w, r, http.StatusUnauthorized, "authentication_required", "autenticação necessária")
		case errors.Is(err, services.ErrURLNotFound):
			problem.Write(w, r, http.StatusNotFound, "link_not_found", "url não encontrada")
		default:
			slog.ErrorContext(ctx, "daily clicks failed", "error", err)
			problem.Write(w, r, http.StatusInternalServerError, "internal_error", "erro interno do servidor")
		}
		return
	}

	response := clickStatsResponse{
		Days:   make([]dayClicksResponse, len(stats.Days)),
		Totals: clickTotalsResponse{Link: stats.Link, QR: stats.QR},
	}
	for i, day := range stats.Days {
		response.Days[i] = dayClicksResponse{Date: day.Day.Format(time.DateOnly), Link: day.Link, QR: day.QR}
	}

	// Clicks keep arriving; the page asks again when it wants fresh numbers.
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		slog.ErrorContext(ctx, "encode daily clicks failed", "error", err)
	}
}
