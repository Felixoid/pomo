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

func TestFilterUnfinishedAndFinished(t *testing.T) {
	tasks := []*Task{
		{ID: 1, NPomodoros: 4, Pomodoros: []*Pomodoro{{}, {}, {}}},         // unfinished: 3/4
		{ID: 2, NPomodoros: 2, Pomodoros: []*Pomodoro{{}, {}}},             // finished: 2/2
		{ID: 3, NPomodoros: 3, Pomodoros: []*Pomodoro{}},                   // unfinished: 0/3
		{ID: 4, NPomodoros: 1, Pomodoros: []*Pomodoro{{}}},                 // finished: 1/1
		{ID: 5, NPomodoros: 5, Pomodoros: []*Pomodoro{{}, {}, {}, {}, {}}}, // finished: 5/5
	}

	// Test Unfinished filter
	unfinished := Unfinished(tasks)
	if len(unfinished) != 2 {
		t.Fatalf("expected 2 unfinished tasks, got %d", len(unfinished))
	}
	if unfinished[0].ID != 1 || unfinished[1].ID != 3 {
		t.Fatalf("expected unfinished task IDs [1, 3], got [%d, %d]", unfinished[0].ID, unfinished[1].ID)
	}

	// Test Finished filter
	finished := Finished(tasks)
	if len(finished) != 3 {
		t.Fatalf("expected 3 finished tasks, got %d", len(finished))
	}
	if finished[0].ID != 2 || finished[1].ID != 4 || finished[2].ID != 5 {
		t.Fatalf("expected finished task IDs [2, 4, 5], got [%d, %d, %d]",
			finished[0].ID, finished[1].ID, finished[2].ID)
	}
}

func TestFilterByTag(t *testing.T) {
	tasks := []*Task{
		{ID: 1, Tags: []string{"work", "urgent"}},
		{ID: 2, Tags: []string{"personal"}},
		{ID: 3, Tags: []string{"work", "review"}},
		{ID: 4, Tags: []string{"urgent", "bug"}},
		{ID: 5, Tags: []string{}},
	}

	// Filter by single tag
	workTasks := WithTag([]string{"work"}, tasks)
	if len(workTasks) != 2 {
		t.Fatalf("expected 2 tasks with 'work' tag, got %d", len(workTasks))
	}
	if workTasks[0].ID != 1 || workTasks[1].ID != 3 {
		t.Fatalf("expected task IDs [1, 3], got [%d, %d]", workTasks[0].ID, workTasks[1].ID)
	}

	// Filter by multiple tags (OR logic)
	multiTags := WithTag([]string{"urgent", "personal"}, tasks)
	if len(multiTags) != 3 {
		t.Fatalf("expected 3 tasks with 'urgent' or 'personal' tag, got %d", len(multiTags))
	}

	// Filter with no results
	noMatch := WithTag([]string{"nonexistent"}, tasks)
	if len(noMatch) != 0 {
		t.Fatalf("expected 0 tasks with 'nonexistent' tag, got %d", len(noMatch))
	}

	// Empty filter returns all
	allTasks := WithTag([]string{}, tasks)
	if len(allTasks) != 5 {
		t.Fatalf("expected 5 tasks with empty filter, got %d", len(allTasks))
	}
}
