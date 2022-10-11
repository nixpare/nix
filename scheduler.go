package nix

import (
	"fmt"
	"runtime/debug"
	"strings"
	"sync"
)

type Scheduler struct {
	name      string
	depth     int
	parent    *sync.WaitGroup
	child     *sync.WaitGroup
	exited    bool
	panicChan chan routinePanic
	errChan   chan routineError
	logger    Logger
}

type routineError struct {
	err error
	sc  *Scheduler
}

type routinePanic struct {
	err   any
	stack string
	sc    *Scheduler
}

func NewScheduler(logger Logger, name string) *Scheduler {
	return &Scheduler{
		name:      name,
		panicChan: make(chan routinePanic, 10),
		errChan:   make(chan routineError, 10),
		logger:    logger,
	}
}

func (sc *Scheduler) Start() {
	go func() {
		for {
			select {
			case rPanic := <-sc.panicChan:
				sc.logger.Log(
					LogLevelFatal,
					fmt.Sprintf("routine panic (%v): %v", rPanic.sc, rPanic.err),
					rPanic.stack,
				)
			case rError := <-sc.errChan:
				sc.logger.Log(
					LogLevelError,
					fmt.Sprintf("routine error (%v): %v", rError.sc, rError.err),
				)
			}
		}
	}()

	sc.Wait()
	close(sc.panicChan)
	close(sc.errChan)
}

func (sc *Scheduler) Wait() {
	if sc.child != nil {
		sc.child.Wait()
	}
}

func (sc *Scheduler) Exit() {
	if !sc.exited {
		sc.exited = true
		sc.parent.Done()
	}
}

func (sc *Scheduler) Name() string {
	if sc.depth == 0 {
		return sc.name
	}

	return fmt.Sprintf("%s(+%d)", sc.name, sc.depth)
}

func (sc *Scheduler) String() string {
	return sc.Name()
}

func (sc *Scheduler) recoverF() {
	if err := recover(); err != nil {
		sc.panicChan <- routinePanic{
			err, stack(), sc,
		}
	}
}

func (sc *Scheduler) deferF() {
	sc.Exit()
}

func stack() string {
	var out string

	split := strings.Split(string(debug.Stack()), "\n")
	cont := true

	for _, s := range split {
		if strings.HasPrefix(s, "panic(") {
			cont = false
		}

		if cont {
			continue
		}

		out += s + "\n"
	}

	return strings.TrimRight(out, "\n")
}

func (sc *Scheduler) newChild(name string) *Scheduler {
	var newName string
	if sc.depth == 0 {
		newName = sc.name
	} else {
		newName = sc.Name()
	}

	var newDepth int
	if name != "" {
		newDepth = 0
		newName += "-" + name
	} else {
		newDepth = sc.depth + 1
	}

	return &Scheduler{
		name:      newName,
		depth:     newDepth,
		parent:    sc.child,
		panicChan: sc.panicChan,
		errChan:   sc.errChan,
	}
}

type RoutineFunc func(sc *Scheduler) error

func (sc *Scheduler) Go(f RoutineFunc, name ...string) {
	if sc.child == nil {
		sc.child = new(sync.WaitGroup)
	}
	sc.child.Add(1)

	childSC := sc.newChild(strings.Join(name, " "))
	go childSC.goChildFunc(f)
}

func (sc *Scheduler) GoNB(f RoutineFunc, name ...string) {
	childSC := sc.newChild(strings.Join(name, " "))
	go childSC.goChildFunc(f)
}

func (sc *Scheduler) goChildFunc(f RoutineFunc) {
	defer sc.deferF()
	defer sc.recoverF()

	err := f(sc)
	if err != nil {
		sc.errChan <- routineError{err, sc}
	}
}
