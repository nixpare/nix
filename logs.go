package nix

import (
	"fmt"
	"html/template"
	"strings"
	"time"

	"github.com/nixpare/logger/v3"
	"github.com/nixpare/nix/utility"
)

func (ctx *Context) Logger() *logger.Logger {
	return ctx.l
}

func (ctx *Context) SetLogger(l *logger.Logger) {
	ctx.l = l
	if ctx.main != nil {
		ctx.main.l = l
	}
}

func (ctx *Context) SetErrorTemplate(t *template.Template) {
	ctx.errTemplate = t
	if ctx.main != nil {
		ctx.main.errTemplate = t
	}
}

func (ctx *Context) SetCustomHostLog(host string) {
	ctx.customHostLog = host
	if ctx.main != nil {
		ctx.main.customHostLog = host
	}
}

func (ctx *Context) DisableLogging() {
	ctx.enableLogging = false
	if ctx.main != nil {
		ctx.main.enableLogging = false
	}
}

func (ctx *Context) DisableErrorCapture() {
	ctx.enableErrorCapture = false
	if ctx.main != nil {
		ctx.main.enableErrorCapture = false
	}
}

func (ctx *Context) IsSecure() bool {
	return ctx.r.TLS != nil
}

// metrics is a collection of parameters to log taken from an HTTP
// connection
type metrics struct {
	Code       int
	Duration   time.Duration
	Written    int64
	RemoteAddr string
}

// getMetrics returns a view of the h captured connection metrics
func (ctx *Context) getMetrics() metrics {
	return metrics{
		Code:     ctx.code,
		Duration: time.Since(ctx.connTime),
		Written:  ctx.written,
		RemoteAddr: ctx.r.RemoteAddr,
	}
}

const (
	http_info_format    = "%s%-21s%s - %s%d %-4s%s %-50s%s - %s%s (%6d ms)%s \u279C %s%s %s(%s)%s"
	http_warning_format = "%s%-21s%s - %s%d %-4s%s %-50s%s - %s%s (%6d ms)%s \u279C %s%s %s(%s)%s \u279C %s%s%s"
	http_error_format   = "%s%-21s%s - %s%d %-4s%s %-50s%s - %s%s (%6d ms)%s \u279C %s%s %s(%s)%s \u279C %s%s%s"
	http_panic_format   = "%s%-21s%s - %s%d %-4s%s %-50s%s - %s%s (%6d ms)%s \u279C %s%s %s(%s)%s \u279C %spanic: %s%s"
)

func getProto(ctx *Context) string {
	var lock string
	if ctx.IsSecure() {
		lock = "🔒"
	} else {
		lock = "🔓"
	}

	return lock + " " + ctx.r.Proto
}

// logHTTPInfo logs http request with an exit code < 400
func (ctx *Context) logHTTPInfo(m metrics) {
	if len(ctx.caputedError.internal) == 0 {
		ctx.l.Printf(logger.LOG_LEVEL_INFO, http_info_format,
			logger.BRIGHT_BLUE_COLOR, m.RemoteAddr, logger.DEFAULT_COLOR,
			logger.BRIGHT_GREEN_COLOR, m.Code,
			ctx.r.Method, logger.DARK_GREEN_COLOR,
			ctx.r.RequestURI, logger.DEFAULT_COLOR,
			logger.BRIGHT_BLACK_COLOR, utility.PrintBytes(int(m.Written)),
			m.Duration.Milliseconds(), logger.DEFAULT_COLOR,
			logger.DARK_CYAN_COLOR, ctx.logHost(),
			logger.BRIGHT_BLACK_COLOR, getProto(ctx), logger.DEFAULT_COLOR,
		)
	} else {
		internal := ctx.caputedError.Internal()

		ctx.l.Printf(logger.LOG_LEVEL_INFO, http_warning_format,
			logger.BRIGHT_BLUE_COLOR, m.RemoteAddr, logger.DEFAULT_COLOR,
			logger.BRIGHT_GREEN_COLOR, m.Code,
			ctx.r.Method, logger.DARK_GREEN_COLOR,
			ctx.r.RequestURI, logger.DEFAULT_COLOR,
			logger.BRIGHT_BLACK_COLOR, utility.PrintBytes(int(m.Written)),
			m.Duration.Milliseconds(), logger.DEFAULT_COLOR,
			logger.DARK_CYAN_COLOR, ctx.logHost(),
			logger.BRIGHT_BLACK_COLOR, getProto(ctx), logger.DEFAULT_COLOR,
			logger.BRIGHT_GREEN_COLOR, internal, logger.DEFAULT_COLOR,
		)
	}
}

// logHTTPWarning logs http request with an exit code >= 400 and < 500
func (ctx *Context) logHTTPWarning(m metrics) {
	internal := ctx.caputedError.Internal()
	if internal == "" {
		internal = strings.TrimSpace(string(ctx.caputedError.Data))
	}

	ctx.l.Printf(logger.LOG_LEVEL_WARNING, http_warning_format,
		logger.BRIGHT_BLUE_COLOR, m.RemoteAddr, logger.DEFAULT_COLOR,
		logger.DARK_YELLOW_COLOR, m.Code,
		ctx.r.Method, logger.DARK_GREEN_COLOR,
		ctx.r.RequestURI, logger.DEFAULT_COLOR,
		logger.BRIGHT_BLACK_COLOR, utility.PrintBytes(int(m.Written)),
		m.Duration.Milliseconds(), logger.DEFAULT_COLOR,
		logger.DARK_CYAN_COLOR, ctx.logHost(),
		logger.BRIGHT_BLACK_COLOR, getProto(ctx), logger.DEFAULT_COLOR,
		logger.DARK_YELLOW_COLOR, internal, logger.DEFAULT_COLOR,
	)
}

// logHTTPError logs http request with an exit code >= 500
func (ctx *Context) logHTTPError(m metrics) {
	internal := ctx.caputedError.Internal()
	if internal == "" {
		internal = strings.TrimSpace(string(ctx.caputedError.Data))
	}

	ctx.l.Printf(logger.LOG_LEVEL_ERROR, http_error_format,
		logger.BRIGHT_BLUE_COLOR, m.RemoteAddr, logger.DEFAULT_COLOR,
		logger.DARK_RED_COLOR, m.Code,
		ctx.r.Method, logger.DARK_GREEN_COLOR,
		ctx.r.RequestURI, logger.DEFAULT_COLOR,
		logger.BRIGHT_BLACK_COLOR, utility.PrintBytes(int(m.Written)),
		m.Duration.Milliseconds(), logger.DEFAULT_COLOR,
		logger.DARK_CYAN_COLOR, ctx.logHost(),
		logger.BRIGHT_BLACK_COLOR, getProto(ctx), logger.DEFAULT_COLOR,
		logger.DARK_RED_COLOR, internal, logger.DEFAULT_COLOR,
	)
}

func (ctx *Context) logHTTPPanic(m metrics) {
	internal := ctx.caputedError.Internal()
	if internal == "" {
		internal = strings.TrimSpace(string(ctx.caputedError.Data))
	}

	ctx.l.Printf(logger.LOG_LEVEL_FATAL, http_panic_format,
		logger.BRIGHT_BLUE_COLOR, m.RemoteAddr, logger.DEFAULT_COLOR,
		logger.DARK_RED_COLOR, m.Code,
		ctx.r.Method, logger.DARK_GREEN_COLOR,
		ctx.r.RequestURI, logger.DEFAULT_COLOR,
		logger.BRIGHT_BLACK_COLOR, utility.PrintBytes(int(m.Written)),
		m.Duration.Milliseconds(), logger.DEFAULT_COLOR,
		logger.DARK_CYAN_COLOR, ctx.logHost(),
		logger.BRIGHT_BLACK_COLOR, getProto(ctx), logger.DEFAULT_COLOR,
		logger.DARK_RED_COLOR, internal, logger.DEFAULT_COLOR,
	)
}

func (ctx *Context) logHost() string {
	if ctx.customHostLog == "" {
		return ctx.r.Host
	}

	return fmt.Sprintf("%s (%s)", ctx.r.Host, ctx.customHostLog)
}
