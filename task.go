package nix

import (
	"fmt"
	"sync"
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
	m         *sync.Mutex
	sc        *Scheduler
	running   bool
	programs  map[string]*program
	tasks     map[string]*Task
	ticker1m  *time.Ticker
	ticker10m *time.Ticker
	ticker30m *time.Ticker
	ticker1h  *time.Ticker
}

func (r *Router) newTaskManager() *TaskManager {
	tm := &TaskManager{
		r, new(sync.Mutex), nil, false,
		make(map[string]*program), make(map[string]*Task),
		time.NewTicker(time.Minute), time.NewTicker(time.Minute * 10),
		time.NewTicker(time.Minute * 30), time.NewTicker(time.Hour),
	}

	tm.sc = NewScheduler(tm.router.Logger, "Task Manager")

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

	return tm
}

func (tm *TaskManager) runTasksWithTimer(timer TaskTimer) {
	/*for _, t := range tm.tasks {
		go func() {
			err := t.execF(tm, t)
		}()
	}*/
}

// Checks if a new program can be created with the giver name. If there is an
// already registered program with the same name, it returns false, otherwise
// it returns true
func (tm *TaskManager) checkProgramName(name string) bool {
	_, exists := tm.programs[name]
	return !exists
}

// Checks if a new task can be created with the giver name. If there is an
// already registered task with the same name, it returns false, otherwise
// it returns true
func (tm *TaskManager) checkTaskName(name string) bool {
	_, exists := tm.tasks[name]
	return !exists
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
	forceQuit   bool
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
func (tm *TaskManager) NewTask(name, displayName string, f TaskInitFunc, timer TaskTimer, forceQuit bool) error {
	if !tm.checkTaskName(name) {
		return fmt.Errorf("taskManager: create: task %s already registered", name)
	}
	startupF, execF, cleanupF := f()

	t := &Task{
		name, displayName,
		startupF, execF, cleanupF,
		timer, forceQuit,
	}

	if t.startupF != nil {
		err := t.startupF(tm, t)
		if err != nil {
			return fmt.Errorf("taskManager: failed initializing task %s: %w", name, err)
		}
	}

	return nil
}
