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
	ErrProgramNotFound = errors.New("taskManager: program not found")
	ErrProgramAlreadyRunning = errors.New("taskManager: start: program already running")
	ErrProgramAlreadyStopped = errors.New("taskManager: stop: program already stopped")
	ErrProgramStartup = errors.New("taskManager: start: error during startup")
	ErrProgramRestart = errors.New("taskManager: restart")
)

type program struct {
	name    string
	dir string
	execName string
	args  []string
	exitC 	chan struct{}
	exec    *exec.Cmd
	redirect *os.File
}

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

	go p.wait()
	return nil
}

func (p *program) wait() {
	if p.exec.Process == nil {
		return
	}

	p.exec.Wait()
	p.exec = nil
	p.exitC <- struct{}{}
}

func (p *program) stop() {
	if p.exec != nil {
		p.exec.Process.Kill()
		<- p.exitC
	}
}

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

	p := &program {
		name: name,
		dir: dir,
		execName: execName,
		args: args,
		exitC: make(chan struct{}),
	}

	if redirect {
		p.redirect = tm.router.logFile
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

	p.stop()
	return nil
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
		p.stop()
	}
}