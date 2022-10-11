package nix

import (
	"fmt"
	"io"
	"os"
	"os/exec"
)

// Wraps the default *exec.Cmd structure and makes easier the
// access to redirect the standard output and check when it terminates.
// Another limitation is that graceful shutdown is not implemented yet
// due to Windows limitations, but will be. It's possible to wait for its
// termination on multiple goroutines by waiting for exitC closure. Both
// in and out can be nil
type program struct {
	name             string
	dir              string
	execName         string
	args             []string
	exitC            chan struct{}
	exec             *exec.Cmd
	lastProcessState *os.ProcessState
	running          bool
	in               io.Reader
	out              io.Writer
}

// Creates a new program with the diven parameters
func newProgram(name, dir string, in io.Reader, out io.Writer, execName string, args ...string) (*program, error) {
	info, err := os.Stat(dir)
	if err != nil {
		return nil, fmt.Errorf("directory %s not found", dir)
	}
	if !info.IsDir() {
		return nil, fmt.Errorf("%s is not a directory", dir)
	}

	return &program{
		name:     name,
		dir:      dir,
		execName: execName,
		args:     args,
		exitC:    make(chan struct{}),
		in:       in,
		out:      out,
	}, nil
}

// Starts the program and with a goroutine waits for its
// termination. It returns an error if there is a problem with
// the creation of the new process, but if something happens during
// the execution it will be reported to che channel returned
func (p *program) start() (error, <-chan error) {
	if p.isRunning() {
		return fmt.Errorf("program %s is already running", p.name), nil
	}

	p.exec = exec.Command(p.execName, p.args...)
	if p.dir != "" {
		p.exec.Dir = p.dir
	}

	p.exec.Stdin = p.in
	p.exec.Stdout = p.out
	p.exec.Stderr = p.out

	err := p.exec.Start()
	if err != nil {
		return err, nil
	}

	p.running = true
	errChan := make(chan error, 1)

	go p.afterStart(errChan)

	return nil, errChan
}

// Waits for the process with the already provided function by *os.Process,
// then closes the exitC channel to segnal its termination
func (p *program) afterStart(errChan chan error) {
	err := p.exec.Wait()
	if err != nil {
		errChan <- fmt.Errorf("failed waiting program %s: %w", p.name, err)
	}

	p.lastProcessState = p.exec.ProcessState
	p.running = false

	close(errChan)
	close(p.exitC)
}

// Waits for the process termination (if running) and returns the last process
// state known
func (p *program) wait() *os.ProcessState {
	for range p.exitC {
	}
	return p.lastProcessState
}

// Gracefully stops the process (not implemented, not just kills)
// and waits for the cleanup
func (p *program) stop() error {
	return p.kill()
}

// Forcibly kills the process and waits for the cleanup
func (p *program) kill() error {
	if !p.isRunning() {
		return fmt.Errorf("program %s already stopped", p.name)
	}

	err := p.exec.Process.Kill()
	if err != nil {
		return fmt.Errorf("failed stopping program %s: %w", p.name, err)
	}

	for range p.exitC {
	}

	return nil
}

// Reports whether the program is still running
func (p *program) isRunning() bool {
	return p.running
}

func (p *program) String() string {
	var state string
	if p.isRunning() {
		state = fmt.Sprintf("Running - %d", p.exec.Process.Pid)
	} else {
		state = "Stopped"
	}
	return fmt.Sprintf("%s (%s)", p.name, state)
}

// NewProgram creates a new program with the given parameters.
// The program name must be a unique one and both in and out can
// be nil. Graceful shut down is not implemented yet due to Windows
// limitations, but will be (not it just calls the Kill method).
// It's possible to wait for its termination on multiple goroutines
// by calling the Wait method.
func (tm *TaskManager) NewProgram(name, dir string, in io.Reader, out io.Writer, execName string, args ...string) error {
	if !tm.checkProgramName(name) {
		return fmt.Errorf("taskManager: program named %s already registered", name)
	}

	p, err := newProgram(name, dir, in, out, execName, args...)
	if err != nil {
		return fmt.Errorf("taskManager: %w", err)
	}

	tm.programs[name] = p
	return nil
}

// Finds if a program with the given name is registered in the programs map
func (tm *TaskManager) findProgram(name string) (*program, error) {
	p, ok := tm.programs[name]
	if !ok {
		return nil, fmt.Errorf("taskManager: program %s not found", name)
	}

	return p, nil
}

// StartProgram starts an already registered program if it's not running.
// This method just waits for the successful start-up of the program, but
// It does not wait for the termination. For this, call the Wait method
func (tm *TaskManager) StartProgram(name string) error {
	p, err := tm.findProgram(name)
	if err != nil {
		return err
	}

	err, errChan := p.start()
	if err != nil {
		return fmt.Errorf("taskManager: %w", err)
	}

	go func() {
		for err := range errChan {
			tm.router.Logger.Log(
				LogLevelError,
				fmt.Sprintf("taskManager: program %s execution error: %v", name, err),
			)
		}
	}()

	return nil
}

// StopProgram gracefully stops the program with the given name
// (not implemented now, just kills the program)
func (tm *TaskManager) StopProgram(name string) error {
	p, err := tm.findProgram(name)
	if err != nil {
		return err
	}

	err = p.stop()
	if err != nil {
		//return fmt.Errorf("taskManager: %w", err)
		return err
	}
	return nil
}

// KillProgram forcibly kills the program with the given name
func (tm *TaskManager) KillProgram(name string) error {
	p, err := tm.findProgram(name)
	if err != nil {
		return err
	}

	err = p.kill()
	if err != nil {
		return fmt.Errorf("taskManager: %w", err)
	}
	return nil
}

// RestartProgram first gracefully stops the program (not implemented,
// see StopProgram method) and then starts it again
func (tm *TaskManager) RestartProgram(name string) error {
	_, err := tm.findProgram(name)
	if err != nil {
		return err
	}

	err = tm.StopProgram(name)
	if err != nil {
		return err
	}

	return tm.StartProgram(name)
}

// WaitProgram waits for the termination of the program and returns
// process information
func (tm *TaskManager) WaitProgram(name string) (*os.ProcessState, error) {
	p, err := tm.findProgram(name)
	if err != nil {
		return nil, err
	}

	return p.wait(), nil
}

// ProgramIsRunning tells if the program is running or not
func (tm *TaskManager) ProgramIsRunning(name string) (bool, error) {
	p, err := tm.findProgram(name)
	if err != nil {
		return false, err
	}

	return p.isRunning(), nil
}

// GetProgramsNames returns a slice containing all the names
// of the registered programs
func (tm *TaskManager) GetProgramsNames() []string {
	names := make([]string, 0, len(tm.programs))

	for name := range tm.programs {
		names = append(names, name)
	}

	return names
}

// StopAllPrograms stops all the running programs registered in the
// TaskManager. In case of errors, they will be logged automatically
// with the Router Logger; if you want to manage the errors manually,
// you should for loop with the use of the method GetProgramsNames
/*
Example:
func() {
	for name := range tm.GetProgramsNames() {
		err := tm.StopProgram(name)
		if err != nil {
			// Manager error
		}
	}
}()
*/
func (tm *TaskManager) StopAllPrograms() {
	for _, p := range tm.programs {
		if p.isRunning() {
			err := p.stop()
			if err != nil {
				tm.router.Logger.Log(
					LogLevelError,
					fmt.Sprintf("taskManager: %v", err),
				)
			}
		}
	}
}
