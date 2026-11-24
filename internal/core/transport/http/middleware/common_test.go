package core_http_middleware

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	core_logger "github.com/Tutiu-Maksim/golang-todoapp/internal/core/logger"
	core_http_response "github.com/Tutiu-Maksim/golang-todoapp/internal/core/transport/http/response"
	"github.com/google/uuid"
	"go.uber.org/zap"
)

func TestCORSPreflight(t *testing.T) {
	handler := CORS()(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Fatal("preflight reached the application handler")
	}))

	for _, tc := range []struct {
		name, origin, allowed string
	}{
		{"local UI", "http://localhost:5050", "http://localhost:5050"},
		{"unrecognized origin", "https://example.com", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodOptions, "/api/v1/tasks", nil)
			req.Header.Set("Origin", tc.origin)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if allow := rec.Header().Get("Access-Control-Allow-Origin"); rec.Code != http.StatusOK || allow != tc.allowed {
				t.Fatalf("preflight status=%d, allowed origin=%q, want 200 and %q", rec.Code, allow, tc.allowed)
			}
			if tc.allowed != "" && !strings.Contains(rec.Header().Get("Access-Control-Allow-Methods"), "PATCH") {
				t.Fatal("preflight does not allow PATCH")
			}
		})
	}
}

func TestRequestIDAndPanicRecovery(t *testing.T) {
	log := &core_logger.Logger{Logger: zap.NewNop()}
	for _, tc := range []struct {
		name string
		id   string
	}{
		{"provided request ID", "client-request-id"},
		{"generated request ID", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var handlerID string
			endpoint := http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
				handlerID = r.Header.Get("X-Request-ID")
				panic("endpoint failed")
			})
			handler := ChainMiddleware(endpoint, CORS(), RequestID(), Logger(log), Trace(), Panic())
			req := httptest.NewRequest(http.MethodGet, "/api/v1/tasks", nil)
			req.Header.Set("X-Request-ID", tc.id)
			rec := httptest.NewRecorder()
			handler.ServeHTTP(rec, req)

			if rec.Code != http.StatusInternalServerError || rec.Header().Get("Content-Type") != "application/json" {
				t.Fatalf("panic response status=%d, headers=%v", rec.Code, rec.Header())
			}
			var body core_http_response.ErrorResponse
			if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil || body.Message == "" {
				t.Fatalf("unexpected panic body %q: %v", rec.Body, err)
			}
			if responseID := rec.Header().Get("X-Request-ID"); responseID != handlerID {
				t.Fatalf("response ID %q differs from handler ID %q", responseID, handlerID)
			}
			if tc.id == "" {
				if _, err := uuid.Parse(handlerID); err != nil {
					t.Fatalf("expected generated UUID, got %q", handlerID)
				}
			} else if handlerID != tc.id {
				t.Fatalf("request ID = %q, want %q", handlerID, tc.id)
			}
		})
	}
}
