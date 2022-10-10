package nix

import (
	"fmt"
	"io"
	"os"
	"time"
)

const (
	LogLevelInfo = iota
	LogLevelDebug
	LogLevelWarning
	LogLevelError
	LogLevelFatal
)

type Log struct {
	level         int
	date          time.Time
	msg           string
	userGenerated bool
}

type Logger struct {
	out  io.Writer
	logs []Log
}

func NewLogger(out io.Writer) *Logger {
	logger := &Logger{}
	logger.logs = make([]Log, 0)

	if out == nil {
		logger.out = os.Stdout
	} else {
		logger.out = out
	}

	return logger
}

func (l Logger) logf(level int, userGenerated bool, format string, a ...any) {
	log := Log{
		level,
		time.Now(),
		fmt.Sprintf(format, a...),
		userGenerated,
	}

	l.logs = append(l.logs, log)
	fmt.Fprintf(l.out, format, a...)
}

func (l Logger) Logf(level int, format string, a ...any) {
	l.logf(level, true, format, a...)
}

func (l Logger) log(level int, userGenerated bool, a ...any) {
	log := Log{
		level,
		time.Now(),
		fmt.Sprint(a...),
		userGenerated,
	}

	l.logs = append(l.logs, log)
	fmt.Fprint(l.out, a...)
}

func (l Logger) Log(level int, a ...any) {
	l.log(level, true, a...)
}

func (l Logger) logln(level int, userGenerated bool, a ...any) {
	log := Log{
		level,
		time.Now(),
		fmt.Sprintln(a...),
		userGenerated,
	}

	l.logs = append(l.logs, log)
	fmt.Fprintln(l.out, a...)
}

func (l Logger) Logln(level int, a ...any) {
	l.logln(level, true, a...)
}
