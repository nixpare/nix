package nix

import (
	"fmt"
	"strings"
	"time"

	"github.com/nixpare/logger/v2"
)

func (ctx *Context) Logger() logger.Logger {
	return ctx.l
}

func (ctx *Context) IsSecure() bool {
	return ctx.r.TLS != nil
}

func (ctx *Context) SetCustomHostLog(host string) {
	ctx.main.customHostLog = host
}

func (ctx *Context) DisableLogging() {
	ctx.disableLogging = true
	ctx.main.disableLogging = true
}

func (ctx *Context) DisableErrorCapture() {
	ctx.disableErrorCapture = true
	ctx.main.disableErrorCapture = true
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
	http_info_format    = "%s%-15s%s - %s%d %-4s%s %-50s%s - %s%10.3f MB (%6d ms)%s \u279C %s%s %s(%s)%s"
	http_warning_format = "%s%-15s%s - %s%d %-4s%s %-50s%s - %s%10.3f MB (%6d ms)%s \u279C %s%s %s(%s)%s \u279C %s%s%s"
	http_error_format   = "%s%-15s%s - %s%d %-4s%s %-50s%s - %s%10.3f MB (%6d ms)%s \u279C %s%s %s(%s)%s \u279C %s%s%s"
	http_panic_format   = "%s%-15s%s - %s%d %-4s%s %-50s%s - %s%10.3f MB (%6d ms)%s \u279C %s%s %s(%s)%s \u279C %spanic: %s%s"
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
	ctx.l.Printf(logger.LOG_LEVEL_INFO, http_info_format,
		logger.BRIGHT_BLUE_COLOR, m.RemoteAddr, logger.DEFAULT_COLOR,
		logger.BRIGHT_GREEN_COLOR, m.Code,
		ctx.r.Method, logger.DARK_GREEN_COLOR,
		ctx.r.RequestURI, logger.DEFAULT_COLOR,
		logger.BRIGHT_BLACK_COLOR, float64(m.Written)/1000000.,
		m.Duration.Milliseconds(), logger.DEFAULT_COLOR,
		logger.DARK_CYAN_COLOR, ctx.logHost(),
		logger.BRIGHT_BLACK_COLOR, getProto(ctx), logger.DEFAULT_COLOR,
	)
}

// logHTTPWarning logs http request with an exit code >= 400 and < 500
func (ctx *Context) logHTTPWarning(m metrics) {
	if ctx.caputedError.Internal == "" {
		ctx.caputedError.Internal = strings.TrimSpace(string(ctx.caputedError.Data))
	}

	ctx.l.Printf(logger.LOG_LEVEL_WARNING, http_warning_format,
		logger.BRIGHT_BLUE_COLOR, m.RemoteAddr, logger.DEFAULT_COLOR,
		logger.DARK_YELLOW_COLOR, m.Code,
		ctx.r.Method, logger.DARK_GREEN_COLOR,
		ctx.r.RequestURI, logger.DEFAULT_COLOR,
		logger.BRIGHT_BLACK_COLOR, float64(m.Written)/1000000.,
		m.Duration.Milliseconds(), logger.DEFAULT_COLOR,
		logger.DARK_CYAN_COLOR, ctx.logHost(),
		logger.BRIGHT_BLACK_COLOR, getProto(ctx), logger.DEFAULT_COLOR,
		logger.DARK_YELLOW_COLOR, ctx.caputedError.Internal, logger.DEFAULT_COLOR,
	)
}

// logHTTPError logs http request with an exit code >= 500
func (ctx *Context) logHTTPError(m metrics) {
	if ctx.caputedError.Internal == "" {
		ctx.caputedError.Internal = strings.TrimSpace(string(ctx.caputedError.Data))
	}

	ctx.l.Printf(logger.LOG_LEVEL_ERROR, http_error_format,
		logger.BRIGHT_BLUE_COLOR, m.RemoteAddr, logger.DEFAULT_COLOR,
		logger.DARK_RED_COLOR, m.Code,
		ctx.r.Method, logger.DARK_GREEN_COLOR,
		ctx.r.RequestURI, logger.DEFAULT_COLOR,
		logger.BRIGHT_BLACK_COLOR, float64(m.Written)/1000000.,
		m.Duration.Milliseconds(), logger.DEFAULT_COLOR,
		logger.DARK_CYAN_COLOR, ctx.logHost(),
		logger.BRIGHT_BLACK_COLOR, getProto(ctx), logger.DEFAULT_COLOR,
		logger.DARK_RED_COLOR, ctx.caputedError.Internal, logger.DEFAULT_COLOR,
	)
}

func (ctx *Context) logHTTPPanic(m metrics) {
	if ctx.caputedError.Internal == "" {
		ctx.caputedError.Internal = strings.TrimSpace(string(ctx.caputedError.Data))
	}

	ctx.l.Printf(logger.LOG_LEVEL_FATAL, http_panic_format,
		logger.BRIGHT_BLUE_COLOR, m.RemoteAddr, logger.DEFAULT_COLOR,
		logger.DARK_RED_COLOR, m.Code,
		ctx.r.Method, logger.DARK_GREEN_COLOR,
		ctx.r.RequestURI, logger.DEFAULT_COLOR,
		logger.BRIGHT_BLACK_COLOR, float64(m.Written)/1000000.,
		m.Duration.Milliseconds(), logger.DEFAULT_COLOR,
		logger.DARK_CYAN_COLOR, ctx.logHost(),
		logger.BRIGHT_BLACK_COLOR, getProto(ctx), logger.DEFAULT_COLOR,
		logger.DARK_RED_COLOR, ctx.caputedError.Internal, logger.DEFAULT_COLOR,
	)
}

func (ctx *Context) logHost() string {
	if ctx.customHostLog == "" {
		return ctx.r.Host
	}

	return fmt.Sprintf("%s (%s)", ctx.r.Host, ctx.customHostLog)
}