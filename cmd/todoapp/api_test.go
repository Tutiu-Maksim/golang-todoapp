package main_test

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	core_logger "github.com/Tutiu-Maksim/golang-todoapp/internal/core/logger"
	core_postgres_pool "github.com/Tutiu-Maksim/golang-todoapp/internal/core/repository/postgres/pool"
	core_http_response "github.com/Tutiu-Maksim/golang-todoapp/internal/core/transport/http/response"
	core_http_server "github.com/Tutiu-Maksim/golang-todoapp/internal/core/transport/http/server"
	statistics_service "github.com/Tutiu-Maksim/golang-todoapp/internal/features/statistics/service"
	statistics_transport_http "github.com/Tutiu-Maksim/golang-todoapp/internal/features/statistics/transport/http"
	task_postgres_repository "github.com/Tutiu-Maksim/golang-todoapp/internal/features/tasks/repository/postgres"
	task_service "github.com/Tutiu-Maksim/golang-todoapp/internal/features/tasks/service"
	tasks_transport_http "github.com/Tutiu-Maksim/golang-todoapp/internal/features/tasks/transport/http"
	user_service "github.com/Tutiu-Maksim/golang-todoapp/internal/features/users/service"
	users_transport_http "github.com/Tutiu-Maksim/golang-todoapp/internal/features/users/transport/http"
	web_repository_fs "github.com/Tutiu-Maksim/golang-todoapp/internal/features/web/repository/file_system"
	web_service "github.com/Tutiu-Maksim/golang-todoapp/internal/features/web/service"
	web_transport_http "github.com/Tutiu-Maksim/golang-todoapp/internal/features/web/transport/http"
	"go.uber.org/zap"
)

func TestAPIRejectsInvalidRequests(t *testing.T) {
	users := user_service.NewUsersService(nil)
	tasks := task_service.NewTasksService(nil)
	statistics := statistics_service.NewStatisticsService(nil)
	router := core_http_server.NewAPIVersionRouter(core_http_server.ApiVersion1)
	router.RegisterRoutes(users_transport_http.NewUserService(&users).Routes()...)
	router.RegisterRoutes(tasks_transport_http.NewTasksHTTPHandler(tasks).Routes()...)
	router.RegisterRoutes(statistics_transport_http.NewStatisticsHTTPHandler(statistics).Routes()...)

	for _, tc := range []struct {
		name   string
		method string
		path   string
		body   string
	}{
		{"malformed JSON", "POST", "/users", `{"full_name":`},
		{"name missing", "POST", "/users", `{}`},
		{"invalid phone digits", "POST", "/users", `{"full_name":"Valid User","phone_number":"+12345678a"}`},
		{"bad user ID", "GET", "/users/nope", ""},
		{"invalid user limit", "GET", "/users?limit=oops", ""},
		{"negative user limit", "GET", "/users?limit=-1", ""},
		{"negative user offset", "GET", "/users?offset=-1", ""},
		{"null name", "PATCH", "/users/1", `{"full_name":null}`},
		{"short name", "PATCH", "/users/1", `{"full_name":"А"}`},
		{"short patch phone", "PATCH", "/users/1", `{"phone_number":"+123"}`},
		{"patch phone without plus", "PATCH", "/users/1", `{"phone_number":"1234567890"}`},
		{"missing task author", "POST", "/tasks", `{"title":"Example"}`},
		{"invalid task author filter", "GET", "/tasks?user_id=nope", ""},
		{"invalid task offset", "GET", "/tasks?offset=nope", ""},
		{"negative task limit", "GET", "/tasks?limit=-1", ""},
		{"negative task offset", "GET", "/tasks?offset=-1", ""},
		{"null title", "PATCH", "/tasks/1", `{"title":null}`},
		{"empty title", "PATCH", "/tasks/1", `{"title":""}`},
		{"empty description", "PATCH", "/tasks/1", `{"description":""}`},
		{"null completion state", "PATCH", "/tasks/1", `{"completed":null}`},
		{"wrong completion type", "PATCH", "/tasks/1", `{"completed":"true"}`},
		{"invalid statistics ID", "GET", "/statistics?user_id=nope", ""},
		{"invalid start date", "GET", "/statistics?from=2025-02-30", ""},
		{"invalid end date", "GET", "/statistics?to=tomorrow", ""},
		{"reversed date range", "GET", "/statistics?from=2025-01-02&to=2025-01-01", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			response := apiRequest(t, router, tc.method, tc.path, tc.body, http.StatusBadRequest)
			body := decodeResponse[core_http_response.ErrorResponse](t, response)
			if !strings.Contains(body.Error, "invalid argument") || body.Message == "" {
				t.Fatalf("unexpected response: %+v", body)
			}
		})
	}
}

func TestMainPage(t *testing.T) {
	root := t.TempDir()
	t.Setenv("PROJECT_ROOT", root)
	t.Chdir(root)
	web := web_transport_http.NewWebHTTPHandler(web_service.NewWebService(web_repository_fs.NewWebRepository()))
	router := core_http_server.NewAPIVersionRouter(core_http_server.ApiVersion1)
	router.RegisterRoutes(web.Routes()...)

	missing := apiRequest(t, router, http.MethodGet, "/", "", http.StatusNotFound)
	if body := decodeResponse[core_http_response.ErrorResponse](t, missing); !strings.Contains(body.Error, "not found") {
		t.Fatalf("unexpected missing-page response: %+v", body)
	}

	filename := filepath.Join(root, "public", "index.html")
	if err := os.MkdirAll(filepath.Dir(filename), 0755); err != nil {
		t.Fatal(err)
	}
	const html = "<!doctype html><title>Todo App</title>"
	if err := os.WriteFile(filename, []byte(html), 0644); err != nil {
		t.Fatal(err)
	}
	for _, projectRoot := range []string{root, filepath.Join(root, "missing"), ""} {
		t.Setenv("PROJECT_ROOT", projectRoot)
		response := apiRequest(t, router, http.MethodGet, "/", "", http.StatusOK)
		if response.Header().Get("Content-Type") != "text/html; charset=utf-8" || response.Body.String() != html {
			t.Fatalf("PROJECT_ROOT=%q: unexpected page response: %q", projectRoot, response.Body)
		}
	}
}

func TestGetTaskErrors(t *testing.T) {
	for _, tc := range []struct {
		name       string
		id         string
		scanErr    error
		wantStatus int
		wantError  string
	}{
		{"invalid ID", "abc", nil, 400, "invalid argument"},
		{"missing task", "1", core_postgres_pool.ErrNoRows, 404, "not found"},
		{"database failure", "1", errors.New("database unavailable"), 500, "database unavailable"},
		{"database timeout", "1", context.DeadlineExceeded, 500, "context deadline exceeded"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := task_postgres_repository.NewTasksRepository(taskErrorPool{scanErr: tc.scanErr})
			handler := tasks_transport_http.NewTasksHTTPHandler(task_service.NewTasksService(repo))
			router := core_http_server.NewAPIVersionRouter(core_http_server.ApiVersion1)
			router.RegisterRoutes(handler.Routes()...)

			response := apiRequest(t, router, http.MethodGet, "/tasks/"+tc.id, "", tc.wantStatus)
			body := decodeResponse[core_http_response.ErrorResponse](t, response)
			if !strings.Contains(body.Error, tc.wantError) {
				t.Fatalf("error = %q, want it to contain %q", body.Error, tc.wantError)
			}
		})
	}
}

type taskErrorPool struct {
	core_postgres_pool.Pool
	scanErr error
}

func (p taskErrorPool) OpTimeout() time.Duration { return time.Second }

func (p taskErrorPool) QueryRow(context.Context, string, ...any) core_postgres_pool.Row {
	return taskErrorRow{p.scanErr}
}

type taskErrorRow struct{ err error }

func (r taskErrorRow) Scan(...any) error { return r.err }

func apiRequest(t *testing.T, handler http.Handler, method, path, body string, status int) *httptest.ResponseRecorder {
	t.Helper()

	req := httptest.NewRequest(method, path, strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req = req.WithContext(core_logger.ToContex(req.Context(), &core_logger.Logger{Logger: zap.NewNop()}))
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, req)
	if response.Code != status {
		t.Fatalf("%s %s: status = %d, want %d. Response: %s", method, path, response.Code, status, response.Body)
	}
	return response
}

func decodeResponse[T any](t *testing.T, response *httptest.ResponseRecorder) T {
	t.Helper()

	if contentType := response.Header().Get("Content-Type"); contentType != "application/json" {
		t.Fatalf("Content-Type = %q, want application/json", contentType)
	}

	var body T
	if err := json.Unmarshal(response.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode response: %v. Response: %s", err, response.Body)
	}
	return body
}
