package domain

import (
	"errors"
	"strings"
	"testing"
	"time"

	core_errors "github.com/Tutiu-Maksim/golang-todoapp/internal/core/errors"
)

func TestUserValidate(t *testing.T) {
	for _, tc := range []struct {
		name     string
		fullName string
		phone    *string
		invalid  bool
	}{
		{"minimum name without phone", "Аня", nil, false},
		{"maximum name and phone", strings.Repeat("я", 100), new("+12345678901234"), false},
		{"minimum phone length", "Аня", new("+123456789"), false},
		{"short name", "Ан", nil, true},
		{"long name", strings.Repeat("я", 101), nil, true},
		{"empty phone", "Аня", new(""), true},
		{"long phone", "Аня", new("+123456789012345"), true},
		{"missing plus sign", "Аня", new("1234567890"), true},
		{"non-digit in phone", "Аня", new("+12345678a"), true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user := User{FullName: tc.fullName, PhoneNumber: tc.phone}
			checkValidation(t, user.Validate(), tc.invalid)
		})
	}
}

func TestTaskValidate(t *testing.T) {
	created := time.Date(2025, 1, 2, 3, 0, 0, 0, time.UTC)
	before := created.Add(-time.Second)

	for _, tc := range []struct {
		name        string
		title       string
		description *string
		completed   bool
		completedAt *time.Time
		invalid     bool
	}{
		{"minimum title without description", "Я", nil, false, nil, false},
		{"maximum title and description", strings.Repeat("я", 100), new(strings.Repeat("я", 1000)), true, &created, false},
		{"empty title", "", nil, false, nil, true},
		{"long title", strings.Repeat("я", 101), nil, false, nil, true},
		{"empty description", "Я", new(""), false, nil, true},
		{"long description", "Я", new(strings.Repeat("я", 1001)), false, nil, true},
		{"completed without timestamp", "Я", nil, true, nil, true},
		{"completed before creation", "Я", nil, true, &before, true},
		{"open with completion timestamp", "Я", nil, false, &created, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := NewTask(1, 1, tc.title, tc.description, tc.completed, created, tc.completedAt, 1)
			checkValidation(t, task.Validate(), tc.invalid)
		})
	}
}

func TestUserApplyPatch(t *testing.T) {
	original := NewUser(4, 1, "Иван Иванов", new("+79998887766"))
	for _, tc := range []struct {
		name  string
		patch UserPatch
	}{
		{"null name", UserPatch{FullName: Nullable[string]{Set: true}}},
		{"invalid phone with valid name", UserPatch{
			FullName:    Nullable[string]{Set: true, Value: new("Новое имя")},
			PhoneNumber: Nullable[string]{Set: true, Value: new("+12345678a")},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			user := original
			checkValidation(t, user.ApplyPatch(tc.patch), true)
			if user != original {
				t.Fatalf("rejected patch changed user: %+v", user)
			}
		})
	}

	user := original
	if err := user.ApplyPatch(UserPatch{PhoneNumber: Nullable[string]{Set: true}}); err != nil {
		t.Fatal(err)
	}
	want := original
	want.PhoneNumber = nil
	if user != want {
		t.Fatalf("removing phone gave %+v, want %+v", user, want)
	}
}

func TestTaskApplyPatch(t *testing.T) {
	original := NewTask(4, 1, "Задача", new("Описание"), false, time.Now().Add(-time.Hour), nil, 3)
	for _, tc := range []struct {
		name  string
		patch TaskPatch
	}{
		{"null title", TaskPatch{Title: Nullable[string]{Set: true}}},
		{"null completion state", TaskPatch{Completed: Nullable[bool]{Set: true}}},
		{"empty description with valid title", TaskPatch{
			Title:       Nullable[string]{Set: true, Value: new("Новое название")},
			Description: Nullable[string]{Set: true, Value: new("")},
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			task := original
			checkValidation(t, task.ApplyPatch(tc.patch), true)
			if task != original {
				t.Fatalf("rejected patch changed task: %+v", task)
			}
		})
	}

	task := original
	if err := task.ApplyPatch(TaskPatch{
		Description: Nullable[string]{Set: true},
		Completed:   Nullable[bool]{Set: true, Value: new(true)},
	}); err != nil {
		t.Fatal(err)
	}
	if !task.Completed || task.CompletedAt == nil || task.CompletedAt.Before(task.CreatedAt) || task.Description != nil {
		t.Fatalf("unexpected completed task: %+v", task)
	}
	if err := task.ApplyPatch(TaskPatch{Completed: Nullable[bool]{Set: true, Value: new(false)}}); err != nil {
		t.Fatal(err)
	}
	want := original
	want.Description = nil
	if task != want {
		t.Fatalf("reopening task gave %+v, want %+v", task, want)
	}
}

func checkValidation(t *testing.T, err error, invalid bool) {
	t.Helper()
	if invalid && !errors.Is(err, core_errors.ErrInvalidArgument) {
		t.Fatalf("error = %v, want ErrInvalidArgument", err)
	}
	if !invalid && err != nil {
		t.Fatalf("unexpected validation error: %v", err)
	}
}
