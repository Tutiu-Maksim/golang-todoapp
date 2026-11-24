package statistics_service

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Tutiu-Maksim/golang-todoapp/internal/core/domain"
	core_errors "github.com/Tutiu-Maksim/golang-todoapp/internal/core/errors"
)

func TestGetStatistics(t *testing.T) {
	from := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
	to := from.Add(24 * time.Hour)
	userID := 7
	open := domain.Task{CreatedAt: from}
	fast := domain.Task{CreatedAt: from, Completed: true, CompletedAt: new(from.Add(time.Hour))}
	slow := domain.Task{CreatedAt: from, Completed: true, CompletedAt: new(from.Add(3 * time.Hour))}

	for _, tc := range []struct {
		name        string
		tasks       []domain.Task
		completed   int
		wantRate    *float64
		wantAverage *time.Duration
	}{
		{name: "empty result"},
		{name: "only open tasks", tasks: []domain.Task{open}, wantRate: new(0.0)},
		{name: "one completed task", tasks: []domain.Task{open, fast}, completed: 1,
			wantRate: new(50.0), wantAverage: new(time.Hour)},
		{name: "average across completed tasks", tasks: []domain.Task{open, open, fast, slow}, completed: 2,
			wantRate: new(50.0), wantAverage: new(2 * time.Hour)},
	} {
		t.Run(tc.name, func(t *testing.T) {
			repo := statisticsRepoFunc(func(_ context.Context, gotUser *int, gotFrom, gotTo *time.Time) ([]domain.Task, error) {
				if gotUser == nil || *gotUser != userID || gotFrom == nil || !gotFrom.Equal(from) || gotTo == nil || !gotTo.Equal(to) {
					t.Fatalf("unexpected filters: user=%v, from=%v, to=%v", gotUser, gotFrom, gotTo)
				}
				return tc.tasks, nil
			})
			got, err := NewStatisticsService(repo).GetStatistics(t.Context(), &userID, &from, &to)
			if err != nil {
				t.Fatal(err)
			}
			if got.TasksCreated != len(tc.tasks) || got.TasksCompleted != tc.completed {
				t.Fatalf("created=%d and completed=%d, want %d and %d",
					got.TasksCreated, got.TasksCompleted, len(tc.tasks), tc.completed)
			}
			checkOptionalValue(t, "completion rate", got.TasksCompletedRate, tc.wantRate)
			checkOptionalValue(t, "average duration", got.TasksAverageCompletionTime, tc.wantAverage)
		})
	}
}

func TestGetStatisticsErrors(t *testing.T) {
	t.Run("equal endpoints", func(t *testing.T) {
		date := time.Date(2025, 1, 2, 0, 0, 0, 0, time.UTC)
		service := NewStatisticsService(nil)
		if _, err := service.GetStatistics(t.Context(), nil, &date, &date); !errors.Is(err, core_errors.ErrInvalidArgument) {
			t.Fatalf("error = %v, want ErrInvalidArgument", err)
		}
	})

	t.Run("repository failure", func(t *testing.T) {
		want := errors.New("repository unavailable")
		repo := statisticsRepoFunc(func(context.Context, *int, *time.Time, *time.Time) ([]domain.Task, error) {
			return nil, want
		})
		if _, err := NewStatisticsService(repo).GetStatistics(t.Context(), nil, nil, nil); !errors.Is(err, want) {
			t.Fatalf("error = %v, want %v", err, want)
		}
	})
}

type statisticsRepoFunc func(context.Context, *int, *time.Time, *time.Time) ([]domain.Task, error)

func (f statisticsRepoFunc) GetTasks(ctx context.Context, userID *int, from, to *time.Time) ([]domain.Task, error) {
	return f(ctx, userID, from, to)
}

func checkOptionalValue[T comparable](t *testing.T, name string, got, want *T) {
	t.Helper()
	switch {
	case got != nil && want != nil:
		if *got != *want {
			t.Errorf("%s = %v, want %v", name, *got, *want)
		}
	case got != nil:
		t.Errorf("%s = %v, want nil", name, *got)
	case want != nil:
		t.Errorf("%s = nil, want %v", name, *want)
	}
}
