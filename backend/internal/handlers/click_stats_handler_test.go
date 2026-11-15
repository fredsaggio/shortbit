package handlers_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
	"uuid"

	"github.com/fredsaggio/url-shortener/internal/ctxval"
	"github.com/fredsaggio/url-shortener/internal/handlers"
	"github.com/fredsaggio/url-shortener/internal/problem/problemtest"
	"github.com/fredsaggio/url-shortener/internal/services"
)

type clickStatsServiceStub func(ctx context.Context, userID uuid.UUID, shortCode string, days int) (services.ClickStats, error)

func (f clickStatsServiceStub) DailyClicks(ctx context.Context, userID uuid.UUID, shortCode string, days int) (services.ClickStats, error) {
	return f(ctx, userID, shortCode, days)
}

func statsRequest(t *testing.T, target string) *http.Request {
	t.Helper()
	request := httptest.NewRequestWithContext(ctxval.ContextWithUserID(t.Context(), uuid.NewV7()), http.MethodGet, target, nil)
	request.SetPathValue("code", "Ab3dX9")
	return request
}

func TestClickStatsHandler(t *testing.T) {
	var gotDays int
	handler := handlers.NewClickStatsHandler(clickStatsServiceStub(func(_ context.Context, _ uuid.UUID, code string, days int) (services.ClickStats, error) {
		gotDays = days
		if code != "Ab3dX9" {
			t.Errorf("code = %q", code)
		}
		return services.ClickStats{
			Days: []services.DayClicks{
				{Day: time.Date(2026, 11, 13, 0, 0, 0, 0, time.UTC), Link: 2},
				{Day: time.Date(2026, 11, 14, 0, 0, 0, 0, time.UTC), Link: 1, QR: 4},
			},
			Link: 3,
			QR:   4,
		}, nil
	}))

	response := httptest.NewRecorder()
	handler.DailyClicks(response, statsRequest(t, "/urls/Ab3dX9/clicks?days=7"))

	if response.Code != http.StatusOK || gotDays != 7 {
		t.Fatalf("status = %d, days = %d; want 200 and 7", response.Code, gotDays)
	}
	if response.Header().Get("Cache-Control") != "no-store" {
		t.Error("click stats can be cached")
	}
	var body struct {
		Days []struct {
			Date string `json:"date"`
			Link int64  `json:"link"`
			QR   int64  `json:"qr"`
		} `json:"days"`
		Totals struct {
			Link int64 `json:"link"`
			QR   int64 `json:"qr"`
		} `json:"totals"`
	}
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Days) != 2 || body.Days[1].Date != "2026-11-14" || body.Days[1].QR != 4 || body.Totals.Link != 3 || body.Totals.QR != 4 {
		t.Errorf("body = %+v", body)
	}

	// No period means 30 days.
	handler.DailyClicks(httptest.NewRecorder(), statsRequest(t, "/urls/Ab3dX9/clicks"))
	if gotDays != 30 {
		t.Errorf("default days = %d, want 30", gotDays)
	}
}

func TestClickStatsHandlerMapsErrors(t *testing.T) {
	tests := []struct {
		name       string
		target     string
		err        error
		wantStatus int
		wantCode   string
	}{
		{name: "not a number", target: "/urls/Ab3dX9/clicks?days=muitos", wantStatus: http.StatusBadRequest, wantCode: "invalid_click_period"},
		{name: "other period", target: "/urls/Ab3dX9/clicks?days=14", err: services.ErrInvalidClickPeriod, wantStatus: http.StatusBadRequest, wantCode: "invalid_click_period"},
		{name: "not the owner", target: "/urls/Ab3dX9/clicks", err: services.ErrURLNotFound, wantStatus: http.StatusNotFound, wantCode: "link_not_found"},
		{name: "database down", target: "/urls/Ab3dX9/clicks", err: errors.New("database unavailable"), wantStatus: http.StatusInternalServerError, wantCode: "internal_error"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			handler := handlers.NewClickStatsHandler(clickStatsServiceStub(func(context.Context, uuid.UUID, string, int) (services.ClickStats, error) {
				return services.ClickStats{}, tt.err
			}))
			response := httptest.NewRecorder()
			handler.DailyClicks(response, statsRequest(t, tt.target))

			details, ok := problemtest.Decode(response.Body.String())
			if response.Code != tt.wantStatus || !ok || details.Code != tt.wantCode {
				t.Errorf("response = %d %+v, want %d %s", response.Code, details, tt.wantStatus, tt.wantCode)
			}
		})
	}
}
