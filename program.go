package nix

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
)

var (
	ErrProgramNameAlreadyFound = errors.New("taskManager: create: a program with this name already registered")
	ErrInvalidWorkingDirectory = errors.New("taskManager: create: invalid working directory")
	ErrProgramNotFound         = errors.New("taskManager: program not found")
	ErrProgramAlreadyRunning   = errors.New("taskManager: start: program already running")
	ErrProgramAlreadyStopped   = errors.New("taskManager: stop: program already stopped")
	ErrProgramStop             = errors.New("taskManager: stop: error during stop")
	ErrProgramWait             = errors.New("taskManager: stop: error waiting program to exit")
	ErrProgramStartup          = errors.New("taskManager: start: error during startup")
	ErrProgramRestart          = errors.New("taskManager: restart")
)

// Wraps the default *exec.Cmd structure and makes easier the
// access to redirect the standard output and check when it closes.
// It's not supposed to run programs that necessitate any input from
// Standard Input. Another limitation is that graceful shutdown is not
// implemented yet due to Windows limitations
type program struct {
	tm       *TaskManager
	name     string
	dir      string
	execName string
	args     []string
	exitC    chan struct{}
	exec     *exec.Cmd
	redirect *os.File
}

// Starts the program and with a goroutine waits for its
// termination
func (p *program) start() error {
	p.exec = exec.Command(p.execName, p.args...)
	if p.dir != "" {
		p.exec.Dir = p.dir
	}

	if p.redirect != nil {
		p.exec.Stdout = p.redirect
		p.exec.Stderr = p.redirect
	}

	err := p.exec.Start()
	if err != nil {
		p.exec = nil
		return err
	}

	go func() {
		err := p.wait()
		if err != nil {
			p.tm.router.Logger.logln(LogLevelError, false, err)
		}
	}()
	return nil
}

// Waits for the process with the already provided function by *os.Process,
// then sends a signal to the channel in order to be listened
func (p *program) wait() error {
	if p.exec.Process == nil {
		return ErrProgramAlreadyStopped
	}

	err := p.exec.Wait()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProgramWait, err)
	}

	p.exec = nil
	p.exitC <- struct{}{}
	return nil
}

// Forcibly kills the process
func (p *program) stop() error {
	if p.exec == nil {
		return ErrProgramAlreadyStopped
	}

	err := p.exec.Process.Kill()
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProgramStop, err)
	}

	<-p.exitC
	return nil
}

// Reports whether the program is still running
func (p *program) isOnline() bool {
	return p.exec != nil
}

func (p *program) String() string {
	var state string
	if p.isOnline() {
		state = fmt.Sprintf("Running - %d", p.exec.Process.Pid)
	} else {
		state = "Stopped"
	}
	return fmt.Sprintf("%s (%s)", p.name, state)
}

// NewProgram creates
func (tm *TaskManager) NewProgram(name, dir string, redirect bool, execName string, args ...string) error {
	if !tm.checkProgramName(name) {
		return ErrProgramNameAlreadyFound
	}

	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("%w: not found", ErrInvalidWorkingDirectory)
	}
	if !info.IsDir() {
		return fmt.Errorf("%w: dir is not a directory", ErrInvalidWorkingDirectory)
	}

	p := &program{
		tm:       tm,
		dir:      dir,
		execName: execName,
		args:     args,
		exitC:    make(chan struct{}),
	}

	if redirect {
		if f, ok := tm.router.Logger.out.(*os.File); ok {
			p.redirect = f
		} else {
			return fmt.Errorf("tm: create: redirect not valid: router logger output is not of type *os.File")
		}
	}

	tm.programs[name] = p
	return nil
}

func (tm *TaskManager) StartProgram(name string) error {
	p, ok := tm.programs[name]
	if !ok {
		return fmt.Errorf("%w with name %s", ErrProgramNotFound, name)
	}

	if p.isOnline() {
		return fmt.Errorf("%w - %s", ErrProgramAlreadyRunning, name)
	}

	err := p.start()
	if err != nil {
		return fmt.Errorf("%w: %s - %v", ErrProgramStartup, p.name, err)
	}

	return nil
}

func (tm *TaskManager) StopProgram(name string) error {
	p, ok := tm.programs[name]
	if !ok {
		return fmt.Errorf("%w with name %s", ErrProgramNotFound, name)
	}

	if p.exec == nil {
		return fmt.Errorf("%w - %s", ErrProgramAlreadyStopped, name)
	}

	return p.stop()
}

func (tm *TaskManager) RestartExec(name string) error {
	_, ok := tm.programs[name]
	if !ok {
		return fmt.Errorf("%w with name %s", ErrProgramNotFound, name)
	}

	err := tm.StopProgram(name)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProgramRestart, errors.Unwrap(err))
	}

	err = tm.StartProgram(name)
	if err != nil {
		return fmt.Errorf("%w: %v", ErrProgramRestart, errors.Unwrap(err))
	}

	return nil
}

func (tm *TaskManager) ProgramIsRunning(name string) (bool, error) {
	p, ok := tm.programs[name]
	if !ok {
		return false, fmt.Errorf("%w with name %s", ErrProgramNotFound, name)
	}

	return p.isOnline(), nil
}

func (tm *TaskManager) StopAllPrograms() {
	for _, p := range tm.programs {
		err := p.stop()
		if err != nil {
			tm.router.Logger.logln(LogLevelError, false, err)
		}
	}
}
