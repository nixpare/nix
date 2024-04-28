package nix

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/nixpare/logger/v2"
	"github.com/nixpare/nix/middleware"
)

type mainNixContextKey string
const main_nix_context_key mainNixContextKey = "github.com/nixpare/nix.Context"

func GetMain(r *http.Request) *Context {
	main, ok := r.Context().Value(main_nix_context_key).(*Context)
	if !ok {
		return nil
	}
	return main
}

func setMain(r *http.Request, ctx *Context) {
	*r = *(r.Clone(context.WithValue(r.Context(), main_nix_context_key, ctx)))
}

type Context struct {
	main *Context
	
	w http.ResponseWriter

	r *http.Request

	l logger.Logger

    customHostLog string

	connTime time.Time

	enableLogging bool

	enableErrorCapture bool

	enableRecovery bool

	caputedError CapturedError

	errTemplate *template.Template

	code int

	written int64

	cookieManager *middleware.CookieManager

	cache *middleware.Cache
}

// Header is the equivalent of the http.ResponseWriter method
func (ctx *Context) Header() http.Header {
	return ctx.w.Header()
}

// Write is the equivalent of the http.ResponseWriter method
func (ctx *Context) Write(data []byte) (int, error) {
	if ctx.written == 0 && ctx.code == 0 {
		ctx.WriteHeader(http.StatusOK)
	}

	if ctx.code >= 400 && ctx.enableErrorCapture {
		ctx.caputedError.Data = append(ctx.caputedError.Data, data...)
		return len(data), nil
	}

	n, err := ctx.w.Write(data)
	ctx.written += int64(n)

	return n, err
}

// WriteHeader is the equivalent of the http.ResponseWriter method
// but handles multiple calls, using only the first one used
func (ctx *Context) WriteHeader(statusCode int) {
	if ctx.code != 0 {
		ctx.l.Printf(logger.LOG_LEVEL_WARNING, "Redundant WriteHeader call with code %d", statusCode)
		return
	}

	ctx.code = statusCode
	if statusCode < 400 || !ctx.enableErrorCapture {
		ctx.w.WriteHeader(statusCode)
	} else {
		ctx.caputedError.Code = statusCode
	}
}

func (ctx *Context) Main() *Context {
	if ctx.main != nil {
		return ctx.main
	}

	return GetMain(ctx.r)
}

func (ctx *Context) W() http.ResponseWriter {
	return ctx.w
}

func (ctx *Context) R() *http.Request {
	return ctx.r
}

func serveContext(ctx *Context, handlerFunc func(*Context)) {
	if ctx.enableRecovery {
		panicErr := logger.CapturePanic(func() error {
			handlerFunc(ctx)
			return nil
		})
	
		if panicErr != nil {
			if ctx.code == 0 {
				ctx.Error(http.StatusInternalServerError, "Internal server error", panicErr)
				if ctx.written == 0 {
					ctx.serveError()
				}
			} else {
				if ctx.written == 0 {
					ctx.serveError()
				}
	
				if len(ctx.caputedError.internal) == 0 {
					ctx.caputedError.internal = []string{fmt.Sprintf("panic after response: %v", panicErr)}
				} else {
					ctx.caputedError.internal = []string{fmt.Sprintf(
						"panic after response: %v -> response error: %s\n%s",
						panicErr.Unwrap(),
						ctx.caputedError.Internal(),
						panicErr.Stack(),
					)}
				}
			}
	
			ctx.logHTTPPanic(ctx.getMetrics())
			return
		}
	} else {
		handlerFunc(ctx)
	}

	if ctx.code == 0 {
		ctx.WriteHeader(http.StatusOK)
	}

	if ctx.code >= 400 && ctx.enableErrorCapture {
		ctx.serveError()
	}

	if !ctx.enableLogging {
		return
	}

	metrics := ctx.getMetrics()

	switch {
	case metrics.Code < 400:
		ctx.logHTTPInfo(metrics)
	case metrics.Code >= 400 && metrics.Code < 500:
		ctx.logHTTPWarning(metrics)
	default:
		ctx.logHTTPError(metrics)
	}
}
