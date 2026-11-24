package main_test

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/Tutiu-Maksim/golang-todoapp/internal/core/domain"
	core_errors "github.com/Tutiu-Maksim/golang-todoapp/internal/core/errors"
	core_pgx_pool "github.com/Tutiu-Maksim/golang-todoapp/internal/core/repository/postgres/pool/pgx"
	core_http_server "github.com/Tutiu-Maksim/golang-todoapp/internal/core/transport/http/server"
	statistics_postgres_repository "github.com/Tutiu-Maksim/golang-todoapp/internal/features/statistics/repository/postgres"
	statistics_service "github.com/Tutiu-Maksim/golang-todoapp/internal/features/statistics/service"
	statistics_transport_http "github.com/Tutiu-Maksim/golang-todoapp/internal/features/statistics/transport/http"
	task_postgres_repository "github.com/Tutiu-Maksim/golang-todoapp/internal/features/tasks/repository/postgres"
	task_service "github.com/Tutiu-Maksim/golang-todoapp/internal/features/tasks/service"
	tasks_transport_http "github.com/Tutiu-Maksim/golang-todoapp/internal/features/tasks/transport/http"
	users_postgres_repository "github.com/Tutiu-Maksim/golang-todoapp/internal/features/users/repository/postgres"
	user_service "github.com/Tutiu-Maksim/golang-todoapp/internal/features/users/service"
	users_transport_http "github.com/Tutiu-Maksim/golang-todoapp/internal/features/users/transport/http"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
)

func TestPostgresAPI(t *testing.T) {
	pool := openTestPostgres(t)
	applyMigration(t, pool, "000001_init.up.sql")
	verifyDescriptionMigration(t, pool)

	usersRepo := users_postgres_repository.NewUserRepository(pool)
	users := user_service.NewUsersService(usersRepo)
	tasksRepo := task_postgres_repository.NewTasksRepository(pool)
	tasks := task_service.NewTasksService(tasksRepo)
	statistics := statistics_service.NewStatisticsService(statistics_postgres_repository.NewStatisticsRepository(pool))

	router := core_http_server.NewAPIVersionRouter(core_http_server.ApiVersion1)
	router.RegisterRoutes(users_transport_http.NewUserService(&users).Routes()...)
	router.RegisterRoutes(tasks_transport_http.NewTasksHTTPHandler(tasks).Routes()...)
	router.RegisterRoutes(statistics_transport_http.NewStatisticsHTTPHandler(statistics).Routes()...)
	api := http.StripPrefix("/api/v1", router)
	request := func(method, path, body string, status int) *httptest.ResponseRecorder {
		return apiRequest(t, api, method, "/api/v1"+path, body, status)
	}

	const phone = "+79998887766"
	user := decodeResponse[users_transport_http.UserDTOResponse](t, request("POST", "/users",
		`{"full_name":"Анна Смирнова","phone_number":"+79998887766"}`, 201))
	if user.ID == 0 || user.Version != 1 {
		t.Fatalf("unexpected new user: %+v", user)
	}

	userPath := fmt.Sprintf("/users/%d", user.ID)
	got := decodeResponse[users_transport_http.UserDTOResponse](t, request("GET", userPath, "", 200))
	if got.FullName != user.FullName || got.PhoneNumber == nil || *got.PhoneNumber != phone {
		t.Fatalf("GET user = %+v, want name %q and phone %q", got, user.FullName, phone)
	}
	renamed := decodeResponse[users_transport_http.UserDTOResponse](t, request("PATCH", userPath,
		`{"full_name":"Анна Иванова"}`, 200))
	if renamed.Version != 2 || renamed.FullName != "Анна Иванова" || renamed.PhoneNumber == nil || *renamed.PhoneNumber != phone {
		t.Fatalf("unexpected renamed user: %+v", renamed)
	}
	stale := domain.NewUser(user.ID, user.Version, user.FullName, user.PhoneNumber)
	if _, err := usersRepo.PatchUser(t.Context(), user.ID, stale); !errors.Is(err, core_errors.ErrConflict) {
		t.Fatalf("stale version update error = %v, want conflict", err)
	}
	updated := decodeResponse[users_transport_http.UserDTOResponse](t, request("PATCH", userPath,
		`{"phone_number":null}`, 200))
	got = decodeResponse[users_transport_http.UserDTOResponse](t, request("GET", userPath, "", 200))
	if updated.Version != 3 || updated.PhoneNumber != nil || got.Version != updated.Version || got.PhoneNumber != nil {
		t.Fatalf("unexpected user after removing phone: PATCH %+v, GET %+v", updated, got)
	}

	userList := decodeResponse[users_transport_http.GetUsersResponse](t, request("GET", "/users?limit=1&offset=1", "", 200))
	if len(userList) != 1 || userList[0].ID != user.ID {
		t.Fatalf("unexpected user list: %+v", userList)
	}

	task := decodeResponse[tasks_transport_http.TaskDTOResponse](t, request("POST", "/tasks",
		fmt.Sprintf(`{"title":"Проверить API","description":"Интеграционный тест","author_user_id":%d}`, user.ID), 201))
	taskPath := fmt.Sprintf("/tasks/%d", task.ID)
	readTask := decodeResponse[tasks_transport_http.TaskDTOResponse](t, request("GET", taskPath, "", 200))
	if task.ID == 0 || readTask.Title != "Проверить API" || readTask.AuthorUserID != user.ID ||
		readTask.Description == nil || *readTask.Description != "Интеграционный тест" || readTask.Completed {
		t.Fatalf("unexpected new task: %+v", readTask)
	}
	filter := fmt.Sprintf("?user_id=%d", user.ID)
	taskList := decodeResponse[tasks_transport_http.GetTasksResponse](t, request("GET", "/tasks"+filter, "", 200))
	if len(taskList) != 1 || taskList[0].ID != task.ID {
		t.Fatalf("unexpected task list: %+v", taskList)
	}
	staleTask, err := tasksRepo.GetTask(t.Context(), task.ID)
	if err != nil {
		t.Fatal(err)
	}
	done := decodeResponse[tasks_transport_http.TaskDTOResponse](t, request("PATCH", taskPath,
		`{"completed":true,"description":null}`, 200))
	if done.Version != 2 || !done.Completed || done.CompletedAt == nil || done.Description != nil {
		t.Fatalf("unexpected completed task: %+v", done)
	}
	if _, err := tasksRepo.PatchTask(t.Context(), task.ID, staleTask); !errors.Is(err, core_errors.ErrConflict) {
		t.Fatalf("stale task update error = %v, want conflict", err)
	}
	stats := decodeResponse[statistics_transport_http.GetStatisticsResponse](t, request("GET", "/statistics"+filter, "", 200))
	if stats.TasksCreated != 1 || stats.TasksCompleted != 1 {
		t.Fatalf("unexpected statistics: %+v", stats)
	}

	// The range includes its start and excludes its end, even when both dates match a row.
	_, err = pool.Exec(t.Context(), `
		WITH other_author AS (
			INSERT INTO todoapp.users (full_name) VALUES ('Other author') RETURNING id
		)
		INSERT INTO todoapp.tasks (title, completed, created_at, author_user_id) VALUES
		('Before range', false, '2000-01-01 23:59:59+00', 1),
		('At start', false, '2000-01-02 00:00:00+00', 1),
		('At end', false, '2000-01-03 00:00:00+00', 1),
		('Different author', false, '2000-01-02 00:00:00+00', (SELECT id FROM other_author))`)
	if err != nil {
		t.Fatal(err)
	}
	window := decodeResponse[statistics_transport_http.GetStatisticsResponse](t, request(
		"GET", "/statistics?user_id=1&from=2000-01-02&to=2000-01-03", "", 200))
	if window.TasksCreated != 1 || window.TasksCompleted != 0 {
		t.Fatalf("unexpected statistics for date range and author: %+v", window)
	}
	empty := decodeResponse[statistics_transport_http.GetStatisticsResponse](t, request(
		"GET", "/statistics?to=1999-01-01", "", 200))
	if empty.TasksCreated != 0 || empty.TasksCompleted != 0 ||
		empty.TasksCompletedRate != nil || empty.TasksAverageCompletionTime != nil {
		t.Fatalf("unexpected statistics for empty date range: %+v", empty)
	}
	paged := decodeResponse[tasks_transport_http.GetTasksResponse](t, request("GET", "/tasks?limit=1", "", 200))
	if len(paged) != 1 || paged[0].ID != 1 {
		t.Fatalf("unexpected unfiltered page: %+v", paged)
	}

	request("POST", "/users", `{"full_name":"Анна Иванова","phone_number":"+1234abcd5"}`, 400)
	request("PATCH", userPath, `{"full_name":null}`, 400)
	request("PATCH", taskPath, `{"description":""}`, 400)
	request("POST", "/tasks", `{"title":"Без автора","author_user_id":999999}`, 404)
	reopened := decodeResponse[tasks_transport_http.TaskDTOResponse](t, request("PATCH", taskPath,
		`{"completed":false}`, 200))
	if reopened.Version != 3 || reopened.Completed || reopened.CompletedAt != nil {
		t.Fatalf("unexpected reopened task: %+v", reopened)
	}
	request("DELETE", taskPath, "", 204)
	request("GET", taskPath, "", 404)
	request("DELETE", taskPath, "", 404)
	request("DELETE", userPath, "", 204)
	request("GET", userPath, "", 404)
	request("DELETE", userPath, "", 404)
}

func openTestPostgres(t *testing.T) *core_pgx_pool.Pool {
	t.Helper()

	dsn := os.Getenv("TODOAPP_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TODOAPP_TEST_DATABASE_URL to run PostgreSQL integration tests")
	}

	ctx, cancel := context.WithTimeout(t.Context(), 15*time.Second)
	defer cancel()
	admin, err := pgx.Connect(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := admin.Close(ctx); err != nil {
			t.Errorf("close admin connection: %v", err)
		}
	})

	database := "todoapp_test_" + strings.ToLower(rand.Text())
	identifier := pgx.Identifier{database}.Sanitize()
	if _, err := admin.Exec(ctx, "CREATE DATABASE "+identifier); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if _, err := admin.Exec(ctx, "DROP DATABASE "+identifier); err != nil {
			t.Errorf("drop test database: %v", err)
		}
	})

	cfg := admin.Config()
	pool, err := core_pgx_pool.NewPool(ctx, core_pgx_pool.Config{
		Host: cfg.Host, Port: strconv.Itoa(int(cfg.Port)), User: cfg.User,
		Password: cfg.Password, Database: database, Timeout: 5 * time.Second,
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	return pool
}

func applyMigration(t *testing.T, pool *core_pgx_pool.Pool, filename string) {
	t.Helper()

	script, err := os.ReadFile(filepath.Join("..", "..", "migrations", filename))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(t.Context(), string(script)); err != nil {
		t.Fatalf("apply %s: %v", filename, err)
	}
}

func verifyDescriptionMigration(t *testing.T, pool *core_pgx_pool.Pool) {
	t.Helper()

	ctx := t.Context()
	if _, err := pool.Exec(ctx, `INSERT INTO todoapp.users (full_name) VALUES ('Legacy User')`); err != nil {
		t.Fatal(err)
	}
	const insert = `INSERT INTO todoapp.tasks (title, description, completed, created_at, author_user_id)
		VALUES ('Legacy task', $1, false, now(), 1)`
	if _, err := pool.Exec(ctx, insert, ""); err != nil {
		t.Fatalf("seed task with empty description: %v", err)
	}

	const up = "000002_fix_task_description_check.up.sql"
	applyMigration(t, pool, up)
	var description *string
	if err := pool.QueryRow(ctx, "SELECT description FROM todoapp.tasks WHERE id = 1").Scan(&description); err != nil {
		t.Fatal(err)
	}
	if description != nil {
		t.Fatalf("migrated empty description = %q, want NULL", *description)
	}

	for _, value := range []any{nil, strings.Repeat("я", 1000)} {
		if _, err := pool.Exec(ctx, insert, value); err != nil {
			t.Fatalf("insert valid description: %v", err)
		}
	}
	_, err := pool.Exec(ctx, insert, "")
	var pgErr *pgconn.PgError
	if !errors.As(err, &pgErr) || pgErr.Code != "23514" || pgErr.ConstraintName != "tasks_description_check" {
		t.Fatalf("insert empty description error = %v, want tasks_description_check violation", err)
	}

	applyMigration(t, pool, "000002_fix_task_description_check.down.sql")
	if _, err := pool.Exec(ctx, insert, ""); err != nil {
		t.Fatalf("insert empty description after rollback: %v", err)
	}
	applyMigration(t, pool, up)
}
