package nix

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

// TaskTimer tells the TaskManager how often a Task should be executed.
// See the constants for the values accepted by the TaskManager
type TaskTimer int

const (
	TaskTimer1Minute   TaskTimer = 1  // A Task with this value will be executed every minute
	TaskTimer10Minutes TaskTimer = 10 // A Task with this value will be executed every 10 minutes
	TaskTimer30Minutes TaskTimer = 30 // A Task with this value will be executed every 30 minutes
	TaskTimer1Hour     TaskTimer = 60 // A Task with this value will be executed every hour
	TaskTimerInactive  TaskTimer = -1 // A Task with this value will be never be executed automatically
)

// TaskManager is a component of the Router that controls the execution of external programs
// and tasks registered by the user
type TaskManager struct {
	router    *Router
	exitC     chan struct{}
	sc        *Scheduler
	running   bool
	programs  map[string]*program
	tasks     map[string]*Task
	ticker1m  *time.Ticker
	ticker10m *time.Ticker
	ticker30m *time.Ticker
	ticker1h  *time.Ticker
}

func (r *Router) newTaskManager() {
	r.TaskMgr = &TaskManager{
		r, make(chan struct{}), NewScheduler(r.Logger, "Task Manager"), false,
		make(map[string]*program), make(map[string]*Task),
		time.NewTicker(time.Second * 5), time.NewTicker(time.Minute * 10),
		time.NewTicker(time.Minute * 30), time.NewTicker(time.Hour),
	}
}

func (tm *TaskManager) start() {
	tm.running = true
	go tm.sc.Start()

	go func() {
		for tm.running {
			select {
			case <-tm.ticker1m.C:
				tm.runTasksWithTimer(TaskTimer1Minute)
			case <-tm.ticker10m.C:
				tm.runTasksWithTimer(TaskTimer10Minutes)
			case <-tm.ticker30m.C:
				tm.runTasksWithTimer(TaskTimer30Minutes)
			case <-tm.ticker1h.C:
				tm.runTasksWithTimer(TaskTimer1Hour)
			}
		}
	}()

	for range tm.exitC {
	}

	tm.running = false

	tm.ticker1m.Stop()
	tm.ticker10m.Stop()
	tm.ticker30m.Stop()
	tm.ticker1h.Stop()

	tm.sc.Wait()
}

func (tm *TaskManager) stop() {
	for name := range tm.tasks {
		func(name string) {
			tm.sc.Go(func(routine Routine) error {
				return tm.RemoveTask(name)
			}, name+"(cleanup)")
		}(name)
	}

	tm.sc.Stop()
	close(tm.exitC)
}

func (tm *TaskManager) wait() {
	for range tm.exitC {
	}
}

func (tm *TaskManager) runTasksWithTimer(timer TaskTimer) {
	for _, t := range tm.tasks {
		if t.timer == timer {
			func(t *Task) {
				tm.sc.Go(func(routine Routine) error {
					return t.execF(tm, t)
				}, t.name+("(exec)"))
			}(t)
		}
	}
}

// Checks if a new program can be created with the giver name. If there is an
// already registered program with the same name, it returns false, otherwise
// it returns true
func (tm *TaskManager) checkProgramName(name string) bool {
	_, exists := tm.programs[name]
	return !exists
}

// Checks if there is a task registered with the given name
func (tm *TaskManager) taskNameExists(name string) bool {
	_, exists := tm.tasks[name]
	return exists
}

func (tm *TaskManager) getTask(name string) (*Task, error) {
	if !tm.taskNameExists(name) {
		return nil, fmt.Errorf("taskManager: task %s not found", name)
	}

	return tm.tasks[name], nil
}

// Task can be used like a program to execute periodically (or not)
// a function with the support for a programmed startup and cleanup
// in case the Router has to shut down. A task can be called in execution
// manually and removed from the TaskManager.
// When registered, the task name must be unique, instead the display name
// has no restrictions.
type Task struct {
	name        string
	DisplayName string
	startupF    TaskFunc
	execF       TaskFunc
	cleanupF    TaskFunc
	timer       TaskTimer
}

func (t Task) Name() string {
	return t.name
}

// TaskFunc is the function called by the TaskManager when executing a Task
// function (startup, exec and cleanup). You can access the called Task and
// the entire TaskManager tree
type TaskFunc func(tm *TaskManager, t *Task) error

// TaskInitFunc is used to create a new Task. The implementation is done in this way
// in order to have the possibility to create and access other valiables needed in all
// the functions without any weird caviat.
// Example:
/* func() {
	taskInitF := func() (startupF, execF, cleanupF TaskFunc) {
		var myNeededValiable package.AnyValiabe
		startupF = func(router *nix.Router, t *nix.Task) {
			myNeededVariable = package.InitializeNewValiable()
			// DO SOME OTHER STUFF WITH router AND t
		}
		execF = func(router *nix.Router, t *nix.Task) {
			myNeededVariable.UseValiable()
			// DO SOME OTHER STUFF WITH router AND t
		}
		cleaunpF = func(router *nix.Router, t *nix.Task) {
			// DO SOME OTHER STUFF WITH router AND t
			myNeededVariable.DestroyValiable()
		}
		return
	}
	task := tm.NewTask("myTask", taskInitF, nix.TaskTimerInactive)
}*/
type TaskInitFunc func() (startupF, execF, cleanupF TaskFunc)

// NewTask creates and registers a new Task with the given name, displayName, initialization
// function (f TaskInitFunc) and execution timer, the TaskManager initialize it calling the
// startupF function provided by f (if any). If it returns an error the Task will not be
// registered in the TaskManager.
func (tm *TaskManager) NewTask(name, displayName string, f TaskInitFunc, timer TaskTimer) error {
	if tm.taskNameExists(name) {
		return fmt.Errorf("taskManager: create: task %s already registered", name)
	}
	startupF, execF, cleanupF := f()

	t := &Task{
		name, displayName,
		startupF, execF, cleanupF,
		timer,
	}

	if t.startupF != nil {
		err := PanicToErr(func() error {
			return t.startupF(tm, t)
		})
		if err != nil {
			if panicErr := errors.Unwrap(err); panicErr == nil {
				return fmt.Errorf("taskManager: failed initializing task %s: %w", name, err)
			} else {
				tm.router.Logger.Log(
					LogLevelFatal, panicErr.Error(),
					strings.TrimLeft(err.Error(), panicErr.Error()+"\n"))
				return fmt.Errorf("taskManager: failed initializing task %s: %w", name, panicErr)
			}
		}
	}

	tm.tasks[name] = t
	return nil
}

// ChangeTaskTimer changes the Task timer to the given one
func (tm *TaskManager) ChangeTaskTimer(name string, timer TaskTimer) error {
	t, err := tm.getTask(name)
	if err != nil {
		return err
	}

	t.timer = timer
	return nil
}

// ExecuteTask runs the Task immediatly
func (tm *TaskManager) ExecuteTask(name string, timer TaskTimer) error {
	t, err := tm.getTask(name)
	if err != nil {
		return err
	}

	t.timer = timer
	return nil
}

// RemoveTask runs the cleanup function provided and removes the Task from
// the TaskManager
func (tm *TaskManager) RemoveTask(name string) error {
	t, err := tm.getTask(name)
	if err != nil {
		return err
	}

	if t.cleanupF != nil {
		err = t.cleanupF(tm, t)
	}

	delete(tm.tasks, name)

	return err
}

// GetTasksNames returns all the names of the registered tasks in the
// TaskManager
func (tm *TaskManager) GetTasksNames() []string {
	names := make([]string, 0, len(tm.tasks))
	for name := range tm.tasks {
		names = append(names, name)
	}

	return names
}
