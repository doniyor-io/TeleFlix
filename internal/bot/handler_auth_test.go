package bot

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"tg-movie-bot/config"
)

func TestAdminAuthMiddleware(t *testing.T) {
	handler := NewBotHandler(&BotService{
		cfg: &config.Config{
			BridgeSecret: "admin-secret",
		},
	})

	protected := handler.AdminAuthMiddleware(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	t.Run("rejects missing token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("rejects wrong token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
		req.Header.Set("Authorization", "Bearer wrong")
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusUnauthorized {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusUnauthorized)
		}
	})

	t.Run("accepts valid token", func(t *testing.T) {
		req := httptest.NewRequest(http.MethodGet, "/api/admin/stats", nil)
		req.Header.Set("Authorization", "Bearer admin-secret")
		rec := httptest.NewRecorder()

		protected.ServeHTTP(rec, req)

		if rec.Code != http.StatusNoContent {
			t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
		}
	})
}
