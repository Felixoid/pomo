package cmd

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	pomo "github.com/Felixoid/pomo/pkg/internal"
)

func checkErr(t *testing.T, err error) {
	if err != nil {
		t.Helper()
		t.Fatal(err)
	}
}

func initTestConfig(t *testing.T) (*pomo.Store, *pomo.Config) {
	tmpPath, err := os.MkdirTemp("", "pomo-test-")
	checkErr(t, err)
	t.Cleanup(func() {
		if !t.Failed() {
			if err := os.RemoveAll(tmpPath); err != nil {
				t.Logf("cleanup: %v", err)
			}
		}
	})
	config := &pomo.Config{
		DateTimeFmt: "2006-01-02 15:04",
		BasePath:    tmpPath,
		DBPath:      filepath.Join(tmpPath, "pomo.db"),
		SocketPath:  filepath.Join(tmpPath, "pomo.sock"),
		IconPath:    filepath.Join(tmpPath, "icon.png"),
	}
	store, err := pomo.NewStore(config.DBPath)
	checkErr(t, err)
	checkErr(t, pomo.InitDB(store))
	return store, config
}

func TestPomoCreate(t *testing.T) {
	store, config := initTestConfig(t)
	cmd := New(config)
	checkErr(t, cmd.Run([]string{"pomo", "create", "fuu"}))
	// verify the task was created
	checkErr(t, store.With(func(tx *sql.Tx) error {
		task, err := store.ReadTask(tx, 1)
		checkErr(t, err)
		if task.Message != "fuu" {
			checkErr(t, fmt.Errorf("task should have message fuu, got %s", task.Message))
		}
		return nil
	}))
}

func TestPomoEdit(t *testing.T) {
	store, config := initTestConfig(t)

	// Create a task first
	cmd := New(config)
	checkErr(t, cmd.Run([]string{"pomo", "create", "-d", "25m", "-p", "4", "-t", "test", "original message"}))

	// Edit duration
	cmd = New(config)
	checkErr(t, cmd.Run([]string{"pomo", "edit", "-d", "30m", "1"}))
	checkErr(t, store.With(func(tx *sql.Tx) error {
		task, err := store.ReadTask(tx, 1)
		checkErr(t, err)
		if task.Duration.Minutes() != 30 {
			checkErr(t, fmt.Errorf("duration should be 30m, got %v", task.Duration))
		}
		if task.Message != "original message" {
			checkErr(t, fmt.Errorf("message should be unchanged, got %s", task.Message))
		}
		return nil
	}))

	// Edit message
	cmd = New(config)
	checkErr(t, cmd.Run([]string{"pomo", "edit", "-m", "updated message", "1"}))
	checkErr(t, store.With(func(tx *sql.Tx) error {
		task, err := store.ReadTask(tx, 1)
		checkErr(t, err)
		if task.Message != "updated message" {
			checkErr(t, fmt.Errorf("message should be 'updated message', got %s", task.Message))
		}
		return nil
	}))

	// Edit pomodoros
	cmd = New(config)
	checkErr(t, cmd.Run([]string{"pomo", "edit", "-p", "6", "1"}))
	checkErr(t, store.With(func(tx *sql.Tx) error {
		task, err := store.ReadTask(tx, 1)
		checkErr(t, err)
		if task.NPomodoros != 6 {
			checkErr(t, fmt.Errorf("pomodoros should be 6, got %d", task.NPomodoros))
		}
		return nil
	}))

	// Edit tags
	cmd = New(config)
	checkErr(t, cmd.Run([]string{"pomo", "edit", "-t", "tag1", "-t", "tag2", "1"}))
	checkErr(t, store.With(func(tx *sql.Tx) error {
		task, err := store.ReadTask(tx, 1)
		checkErr(t, err)
		if len(task.Tags) != 2 || task.Tags[0] != "tag1" || task.Tags[1] != "tag2" {
			checkErr(t, fmt.Errorf("tags should be [tag1, tag2], got %v", task.Tags))
		}
		return nil
	}))

	// Edit multiple fields at once
	cmd = New(config)
	checkErr(t, cmd.Run([]string{"pomo", "edit", "-d", "45m", "-p", "8", "-m", "final message", "1"}))
	checkErr(t, store.With(func(tx *sql.Tx) error {
		task, err := store.ReadTask(tx, 1)
		checkErr(t, err)
		if task.Duration.Minutes() != 45 {
			checkErr(t, fmt.Errorf("duration should be 45m, got %v", task.Duration))
		}
		if task.NPomodoros != 8 {
			checkErr(t, fmt.Errorf("pomodoros should be 8, got %d", task.NPomodoros))
		}
		if task.Message != "final message" {
			checkErr(t, fmt.Errorf("message should be 'final message', got %s", task.Message))
		}
		return nil
	}))
}

func TestPomoEditErrors(t *testing.T) {
	store, config := initTestConfig(t)

	// Create a task
	cmd := New(config)
	checkErr(t, cmd.Run([]string{"pomo", "create", "test task"}))

	// Test editing non-existent task - should error at store level
	err := store.With(func(tx *sql.Tx) error {
		_, err := store.ReadTask(tx, 999)
		return err
	})
	if err == nil {
		t.Fatal("expected error when reading non-existent task")
	}
	if err.Error() != "sql: no rows in result set" {
		t.Fatalf("expected 'sql: no rows in result set', got %v", err)
	}

	// Test that UpdateTask with no matching rows doesn't error (SQLite behavior)
	// but ReadTask before update should catch non-existent tasks
	duration, _ := time.ParseDuration("25m")
	err = store.With(func(tx *sql.Tx) error {
		return store.UpdateTask(tx, 999, pomo.Task{
			Message:    "test",
			Duration:   duration,
			NPomodoros: 4,
			Tags:       []string{},
		})
	})
	// UpdateTask doesn't error in SQLite when no rows match
	if err != nil {
		t.Fatalf("UpdateTask should not error on non-matching rows: %v", err)
	}
}
