package pomo

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"testing"
	"time"
)

func TestTaskRunner(t *testing.T) {
	baseDir, _ := ioutil.TempDir("/tmp", "")
	store, err := NewStore(path.Join(baseDir, "pomo.db"))
	if err != nil {
		t.Error(err)
	}
	err = InitDB(store)
	if err != nil {
		t.Error(err)
	}
	runner, err := NewMockedTaskRunner(&Task{
		Duration:   time.Second * 2,
		NPomodoros: 2,
		Message:    fmt.Sprint("Test Task"),
	}, store, NoopNotifier{})
	if err != nil {
		t.Error(err)
	}

	runner.Start()

	runner.Toggle()
	runner.Toggle()

	runner.Toggle()
	runner.Toggle()
}

func TestStoreUpdateTask(t *testing.T) {
	baseDir, _ := os.MkdirTemp("", "pomo-test-")
	t.Cleanup(func() {
		if !t.Failed() {
			os.RemoveAll(baseDir)
		}
	})
	store, err := NewStore(path.Join(baseDir, "pomo.db"))
	if err != nil {
		t.Error(err)
	}
	err = InitDB(store)
	if err != nil {
		t.Error(err)
	}
	defer store.Close()

	// Create a task
	var taskID int
	err = store.With(func(tx *sql.Tx) error {
		id, err := store.CreateTask(tx, Task{
			Message:    "original",
			Duration:   25 * time.Minute,
			NPomodoros: 4,
			Tags:       []string{"tag1"},
		})
		if err != nil {
			return err
		}
		taskID = id
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Update the task
	err = store.With(func(tx *sql.Tx) error {
		return store.UpdateTask(tx, taskID, Task{
			Message:    "updated",
			Duration:   30 * time.Minute,
			NPomodoros: 6,
			Tags:       []string{"tag2", "tag3"},
		})
	})
	if err != nil {
		t.Fatal(err)
	}

	// Verify the update
	err = store.With(func(tx *sql.Tx) error {
		task, err := store.ReadTask(tx, taskID)
		if err != nil {
			return err
		}
		if task.Message != "updated" {
			return fmt.Errorf("expected message 'updated', got %s", task.Message)
		}
		if task.Duration != 30*time.Minute {
			return fmt.Errorf("expected duration 30m, got %v", task.Duration)
		}
		if task.NPomodoros != 6 {
			return fmt.Errorf("expected 6 pomodoros, got %d", task.NPomodoros)
		}
		if len(task.Tags) != 2 || task.Tags[0] != "tag2" || task.Tags[1] != "tag3" {
			return fmt.Errorf("expected tags [tag2, tag3], got %v", task.Tags)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}

	// Test updating non-existent task (should succeed but affect 0 rows)
	err = store.With(func(tx *sql.Tx) error {
		return store.UpdateTask(tx, 9999, Task{
			Message:    "nonexistent",
			Duration:   10 * time.Minute,
			NPomodoros: 1,
			Tags:       []string{},
		})
	})
	// SQLite doesn't error on UPDATE with no matching rows
	if err != nil {
		t.Fatal(err)
	}
}
