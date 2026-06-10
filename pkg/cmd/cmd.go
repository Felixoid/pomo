package cmd

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/adrg/xdg"
	cli "github.com/jawher/mow.cli"

	pomo "github.com/Felixoid/pomo/pkg/internal"
)

func maybe(err error) {
	if err != nil {
		fmt.Printf("Error:\n%s\n", err)
		os.Exit(1)
	}
}

func defaultConfigPath() string {
	return path.Join(xdg.ConfigHome, "pomo", "config.json")
}

func parseRange(arg string) (int, int, error) {
	if strings.Contains(arg, ":") {
		split := strings.Split(arg, ":")
		start, err := strconv.ParseInt(split[0], 0, 64)
		if err != nil {
			return -1, -1, err
		}
		end, err := strconv.ParseInt(split[1], 0, 64)
		if err != nil {
			return -1, -1, err
		}
		return int(start), int(end), nil
	}
	n, err := strconv.ParseInt(arg, 0, 64)
	if err != nil {
		return -1, -1, err
	}
	return int(n), int(n), err
}

func start(config *pomo.Config) func(*cli.Cmd) {
	return func(cmd *cli.Cmd) {
		cmd.Spec = "[OPTIONS] MESSAGE"
		var (
			duration  = cmd.StringOpt("d duration", "25m", "duration of each stent")
			pomodoros = cmd.IntOpt("p pomodoros", 4, "number of pomodoros")
			message   = cmd.StringArg("MESSAGE", "", "descriptive name of the given task")
			tags      = cmd.StringsOpt("t tag", []string{}, "tags associated with this task (can be specified multiple times)")
		)
		cmd.Action = func() {
			parsed, err := time.ParseDuration(*duration)
			maybe(err)
			db, err := pomo.NewStore(config.DBPath)
			maybe(err)
			defer func() { maybe(db.Close()) }()
			task := &pomo.Task{
				Message:    *message,
				Tags:       *tags,
				NPomodoros: *pomodoros,
				Duration:   parsed,
			}
			maybe(db.With(func(tx *sql.Tx) error {
				id, err := db.CreateTask(tx, *task)
				if err != nil {
					return err
				}
				task.ID = id
				return nil
			}))
			runner, err := pomo.NewTaskRunner(task, config)
			maybe(err)
			server, err := pomo.NewServer(runner, config)
			maybe(err)
			server.Start()
			defer server.Stop()
			runner.Start()
			pomo.StartUI(runner)
		}
	}
}

func create(config *pomo.Config) func(*cli.Cmd) {
	return func(cmd *cli.Cmd) {
		cmd.Spec = "[OPTIONS] MESSAGE"
		var (
			duration  = cmd.StringOpt("d duration", "25m", "duration of each stent")
			pomodoros = cmd.IntOpt("p pomodoros", 4, "number of pomodoros")
			message   = cmd.StringArg("MESSAGE", "", "descriptive name of the given task")
			tags      = cmd.StringsOpt("t tag", []string{}, "tags associated with this task (can be specified multiple times)")
		)
		cmd.Action = func() {
			parsed, err := time.ParseDuration(*duration)
			maybe(err)
			db, err := pomo.NewStore(config.DBPath)
			maybe(err)
			defer func() { maybe(db.Close()) }()
			task := &pomo.Task{
				Message:    *message,
				Tags:       *tags,
				NPomodoros: *pomodoros,
				Duration:   parsed,
			}
			maybe(db.With(func(tx *sql.Tx) error {
				taskId, err := db.CreateTask(tx, *task)
				if err != nil {
					return err
				}
				fmt.Println(taskId)
				return nil
			}))
		}
	}
}

func begin(config *pomo.Config) func(*cli.Cmd) {
	return func(cmd *cli.Cmd) {
		cmd.Spec = "[OPTIONS] TASK_ID"
		var (
			taskId = cmd.IntArg("TASK_ID", -1, "ID of Pomodoro to begin")
		)

		cmd.Action = func() {
			db, err := pomo.NewStore(config.DBPath)
			maybe(err)
			defer func() { maybe(db.Close()) }()
			var task *pomo.Task
			maybe(db.With(func(tx *sql.Tx) error {
				read, err := db.ReadTask(tx, *taskId)
				if err != nil {
					return err
				}
				task = read
				return nil
			}))
			runner, err := pomo.NewTaskRunner(task, config)
			maybe(err)
			server, err := pomo.NewServer(runner, config)
			maybe(err)
			server.Start()
			defer server.Stop()
			runner.Start()
			pomo.StartUI(runner)
		}
	}
}

func initialize(config *pomo.Config) func(*cli.Cmd) {
	return func(cmd *cli.Cmd) {
		cmd.Spec = "[OPTIONS]"
		cmd.Action = func() {
			db, err := pomo.NewStore(config.DBPath)
			maybe(err)
			defer func() { maybe(db.Close()) }()
			maybe(pomo.InitDB(db))
		}
	}
}

func list(config *pomo.Config) func(*cli.Cmd) {
	return func(cmd *cli.Cmd) {
		cmd.Spec = "[OPTIONS]"
		var (
			asJSON     = cmd.BoolOpt("json", false, "output task history as JSON")
			assend     = cmd.BoolOpt("assend", false, "sort tasks assending in age")
			all        = cmd.BoolOpt("a all", true, "output all tasks regardless of age")
			unfinished = cmd.BoolOpt("u unfinished", false, "show only unfinished tasks")
			finished   = cmd.BoolOpt("f finished", false, "show only finished tasks")
			tags       = cmd.StringsOpt("t tag", []string{}, "filter by tag (can be specified multiple times)")
			limit      = cmd.IntOpt("n limit", 0, "limit the number of results by n")
			duration   = cmd.StringOpt("d duration", "24h", "show tasks within this duration")
		)
		cmd.Action = func() {
			duration, err := time.ParseDuration(*duration)
			maybe(err)
			db, err := pomo.NewStore(config.DBPath)
			maybe(err)
			defer func() { maybe(db.Close()) }()
			maybe(db.With(func(tx *sql.Tx) error {
				tasks, err := db.ReadTasks(tx)
				maybe(err)
				if *assend {
					sort.Sort(sort.Reverse(pomo.ByID(tasks)))
				}
				if !*all {
					tasks = pomo.After(time.Now().Add(-duration), tasks)
				}
				if *unfinished {
					tasks = pomo.Unfinished(tasks)
				}
				if *finished {
					tasks = pomo.Finished(tasks)
				}
				if len(*tags) > 0 {
					tasks = pomo.WithTag(*tags, tasks)
				}
				if *limit > 0 && (len(tasks) > *limit) {
					tasks = tasks[0:*limit]
				}
				if *asJSON {
					maybe(json.NewEncoder(os.Stdout).Encode(tasks))
					return nil
				}
				maybe(err)
				pomo.SummerizeTasks(config, tasks)
				return nil
			}))
		}
	}
}

func edit(config *pomo.Config) func(*cli.Cmd) {
	return func(cmd *cli.Cmd) {
		cmd.Spec = "[OPTIONS] TASK_ID"
		cmd.LongDesc = `
edit an existing task by ID

## Examples:
# edit task duration
pomo edit -d 30m 1
# edit task message
pomo edit -m "updated description" 1
# edit multiple fields
pomo edit -d 45m -p 6 -m "new message" 1
# mark task as finished
pomo edit -f 1
`
		var (
			taskID    = cmd.IntArg("TASK_ID", -1, "ID of task to edit")
			duration  = cmd.StringOpt("d duration", "", "new duration of each stent")
			pomodoros = cmd.IntOpt("p pomodoros", -1, "new number of pomodoros")
			message   = cmd.StringOpt("m message", "", "new descriptive name of the task")
			tags      = cmd.StringsOpt("t tag", []string{}, "new tags associated with this task (can be specified multiple times)")
			finish    = cmd.BoolOpt("f finish", false, "mark task as finished (set required pomodoros to current completed count)")
		)
		cmd.Action = func() {
			if *taskID == -1 {
				fmt.Println("Error: TASK_ID is required")
				os.Exit(1)
			}

			db, err := pomo.NewStore(config.DBPath)
			maybe(err)
			defer func() { maybe(db.Close()) }()

			maybe(db.With(func(tx *sql.Tx) error {
				task, err := db.ReadTask(tx, *taskID)
				if err != nil {
					return fmt.Errorf("failed to read task: %w", err)
				}

				// Store original values for summary
				origDuration := task.Duration
				origPomodoros := task.NPomodoros
				origMessage := task.Message
				origTags := make([]string, len(task.Tags))
				copy(origTags, task.Tags)

				var changes strings.Builder
				updated := false

				if *duration != "" {
					parsed, err := time.ParseDuration(*duration)
					if err != nil {
						return fmt.Errorf("invalid duration: %w", err)
					}
					task.Duration = parsed
					fmt.Fprintf(&changes, "  duration: %v -> %v\n", origDuration, parsed)
					updated = true
				}

				if *pomodoros != -1 {
					task.NPomodoros = *pomodoros
					fmt.Fprintf(&changes, "  pomodoros: %d -> %d\n", origPomodoros, *pomodoros)
					updated = true
				}

				if *message != "" {
					task.Message = *message
					fmt.Fprintf(&changes, "  message: %q -> %q\n", origMessage, *message)
					updated = true
				}

				if len(*tags) > 0 {
					task.Tags = *tags
					fmt.Fprintf(&changes, "  tags: %v -> %v\n", origTags, *tags)
					updated = true
				}

				if *finish {
					task.NPomodoros = len(task.Pomodoros)
					fmt.Fprintf(&changes, "  pomodoros: %d -> %d (marked as finished)\n", origPomodoros, len(task.Pomodoros))
					updated = true
				}

				if !updated {
					return fmt.Errorf("no fields specified for update. Use -d, -p, -m, -t, or -f flags")
				}

				err = db.UpdateTask(tx, *taskID, *task)
				if err != nil {
					return fmt.Errorf("failed to update task: %w", err)
				}

				fmt.Printf("Updated task %d:\n%s", *taskID, changes.String())
				return nil
			}))
		}
	}
}

func _delete(config *pomo.Config) func(*cli.Cmd) {
	return func(cmd *cli.Cmd) {
		cmd.Spec = "[OPTIONS] [TASK_ID...]"
		cmd.LongDesc = `
delete one or more tasks by ID

## Examples:
# delete a single task
pomo delete 1
# delete a range of tasks (1 - 10)
pomo delete 1:10
# delete multiple tasks 5, 10, and 20
pomo delete 5 10 20
`
		var taskIDs = cmd.StringsArg("TASK_ID", nil, "task to delete")
		cmd.Action = func() {

			db, err := pomo.NewStore(config.DBPath)
			maybe(err)
			defer func() { maybe(db.Close()) }()
			maybe(db.With(func(tx *sql.Tx) error {
				for _, expr := range *taskIDs {
					start, end, err := parseRange(expr)
					if err != nil {
						return err
					}
					for i := start; i <= end; i++ {
						err := db.DeleteTask(tx, i)
						if err != nil {
							return err
						}
						fmt.Printf("deleted task %d\n", i)
					}
				}

				return nil
			}))
		}
	}
}

func _status(config *pomo.Config) func(*cli.Cmd) {
	return func(cmd *cli.Cmd) {
		cmd.Spec = "[OPTIONS]"
		var asJSON = cmd.BoolOpt("json", false, "output task history as JSON")
		cmd.Action = func() {
			client, err := pomo.NewClient(config.SocketPath)
			if err != nil {
				if *asJSON {
					maybe(json.NewEncoder(os.Stdout).Encode(pomo.Status{}))
				} else {
					fmt.Println(pomo.FormatStatus(pomo.Status{}))
				}
				return
			}
			defer func() { maybe(client.Close()) }()
			status, err := client.Status()
			maybe(err)

			if *asJSON {

				maybe(json.NewEncoder(os.Stdout).Encode(status))

			} else {
				fmt.Println(pomo.FormatStatus(*status))
			}
		}
	}
}

func _config(config *pomo.Config) func(*cli.Cmd) {
	return func(cmd *cli.Cmd) {
		cmd.Spec = "[OPTIONS]"
		cmd.Action = func() {
			maybe(json.NewEncoder(os.Stdout).Encode(config))
		}
	}
}

func New(config *pomo.Config) *cli.Cli {
	app := cli.App("pomo", "Pomodoro CLI")
	app.LongDesc = "Pomo helps you track what you did, how long it took you to do it, and how much effort you expect it to take."
	app.Spec = "[OPTIONS]"
	var (
		path = app.StringOpt("p path", defaultConfigPath(), "path to the pomo config directory")
	)
	app.Before = func() {
		pomo.InitLogger()
		maybe(pomo.LoadConfig(*path, config))
	}
	app.Version("v version", pomo.Version)
	app.Command("start s", "start a new task", start(config))
	app.Command("init", "initialize the sqlite database", initialize(config))
	app.Command("config cf", "display the current configuration", _config(config))
	app.Command("create c", "create a new task without starting", create(config))
	app.Command("begin b", "begin requested pomodoro", begin(config))
	app.Command("list l", "list historical tasks", list(config))
	app.Command("edit e", "edit a stored task", edit(config))
	app.Command("delete d", "delete a stored task", _delete(config))
	app.Command("status st", "output the current status", _status(config))
	return app
}

func Run() {
	defer pomo.FlushLog(os.Stderr)
	if err := New(&pomo.Config{}).Run(os.Args); err != nil {
		fmt.Fprintf(os.Stderr, "Error:\n%s\n", err)
		os.Exit(1)
	}
}
