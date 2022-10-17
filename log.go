package nix

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"
)

const (
	LogLevelInfo LogLevel = iota
	LogLevelDebug
	LogLevelWarning
	LogLevelError
	LogLevelFatal
)

const timeFormat = "2006-01-02 15:04:05.00"

// LogLevel defines the severity of a Log. See the constants
type LogLevel int

func (l LogLevel) String() string {
	switch l {
	case LogLevelInfo:
		return "Info   "
	case LogLevelDebug:
		return "Debug  "
	case LogLevelWarning:
		return "Warning"
	case LogLevelError:
		return "Error  "
	case LogLevelFatal:
		return "Fatal  "
	default:
		return "  ???  "
	}
}

// Log is the structure that can be will store any log reported
// with Logger. It keeps the error severity level (see the constants)
// the date it was created and the message associated with it (probably
// an error). It also has the optional field "extra" that can be used to
// store additional information
type Log struct {
	Level   LogLevel
	Date    time.Time
	Message string
	Extra   string
}

// JSON returns the Log l in a json-encoded string in form of a
// slice of bytes
func (l Log) JSON() []byte {
	jsonL := struct {
		Level   string    `json:"level"`
		Date    time.Time `json:"date"`
		Message string    `json:"message"`
		Extra   string    `json:"extra"`
	}{
		strings.TrimSpace(l.Level.String()), l.Date,
		l.Message, l.Extra,
	}

	b, _ := json.Marshal(jsonL)
	return b
}

func (l Log) String() string {
	return fmt.Sprintf(
		"[%v] - %v: %s",
		l.Date.Format(timeFormat),
		l.Level, l.Message,
	)
}

// logger is used by the Router and can be used by the user to
// create logs that are both written to the chosen io.Writer (if any)
// and saved locally in memory, so that they can be retreived
// programmatically and used (for example to make a view in a website)
type logger struct {
	main      io.Writer
	secondary io.Writer
	logs      []Log
	m         *sync.Mutex
}

// Logger is used by the Router and can be used by the user to
// create logs that are both written to the chosen io.Writer (if any)
// and saved locally in memory, so that they can be retreived
// programmatically and used (for example to make a view in a website)
type Logger interface {
	Log(level LogLevel, message string, extra ...string)
	Logs() []Log
	JSON() []byte
	out() io.Writer
}

// NewLogger creates a new Logger with the given io.Writer out. The
// secondary argument is optional and tells the Logger where to write ONLY
// the logs with an extra field set. If more elements are passed to the
// secondary argument only the first one will be used
func NewLogger(main io.Writer, secondary ...io.Writer) Logger {
	l := &logger{
		main: main,
		logs: make([]Log, 0),
		m:    new(sync.Mutex),
	}

	if len(secondary) > 0 {
		l.secondary = secondary[0]
	}

	return l
}

// log first creates a new Log, appends it to the list of Logs and then prints it
// to out
func (l logger) log(level LogLevel, message string, extra ...string) {
	l.m.Lock()
	defer l.m.Unlock()

	log := Log{
		level, time.Now(),
		message, strings.Join(extra, ""),
	}
	l.logs = append(l.logs, log)

	if l.main != nil {
		if l.main == l.secondary {
			fmt.Fprintf(l.secondary, "%v\n%s\n", log, log.Extra)
			return
		}

		fmt.Fprintln(l.main, log)
	}

	if log.Extra != "" && l.secondary != nil {
		fmt.Fprintf(l.secondary, "%v\n%s\n", log, log.Extra)
	}
}

// Log creates a new Log with the given arguments as a user-generated log, adds it to
// the list of logs and then prints it to the io.Writer
func (l logger) Log(level LogLevel, message string, extra ...string) {
	l.log(level, message, extra...)
}

// Logs returns the list of logs stored
func (l logger) Logs() []Log {
	logs := make([]Log, 0, len(l.logs))
	for _, log := range l.logs {
		logs = append(logs, log)
	}

	return logs
}

// JSON returns the list of logs stored in JSON format (see Log.JSON() method)
func (l logger) JSON() []byte {
	res := make([]byte, 0)
	res = append(res, []byte("{")...)

	for _, log := range l.logs {
		res = append(res, log.JSON()...)
	}

	res = append(res, []byte("}")...)
	return res
}

func (l logger) out() io.Writer {
	return l.main
}

func WriteLogStart(t time.Time) string {
	return "\n     /\\ /\\ /\\                                            /\\ /\\ /\\" +
		"\n     <> <> <> - [" + t.Format(timeFormat) + "] - SERVER ONLINE - <> <> <>" +
		"\n     \\/ \\/ \\/                                            \\/ \\/ \\/\n\n"
}

func WriteLogClosure(t time.Time) string {
	return "\n     /\\ /\\ /\\                                             /\\ /\\ /\\" +
		"\n     <> <> <> - [" + t.Format(timeFormat) + "] - SERVER OFFLINE - <> <> <>" +
		"\n     \\/ \\/ \\/                                             \\/ \\/ \\/\n\n"
}
