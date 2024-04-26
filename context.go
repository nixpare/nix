package nix

import (
	"context"
	"fmt"
	"html/template"
	"net/http"
	"time"

	"github.com/nixpare/logger/v2"
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

	disableLogging bool

	disableErrorCapture bool

	caputedError CapturedError

	errTemplate *template.Template

	code int

	written int64
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

	if ctx.code >= 400 && !ctx.disableErrorCapture {
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
	if statusCode < 400 || ctx.disableErrorCapture {
		ctx.w.WriteHeader(statusCode)
	} else {
		ctx.caputedError.Code = statusCode
	}
}

func (ctx *Context) W() http.ResponseWriter {
	return ctx.w
}

func (ctx *Context) R() *http.Request {
	return ctx.r
}

func serveContext(ctx *Context, handler func(*Context)) {
	panicErr := logger.CapturePanic(func() error {
		handler(ctx)
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

			if ctx.caputedError.Internal == "" {
				ctx.caputedError.Internal = fmt.Sprintf("panic after response: %v", panicErr)
			} else {
				ctx.caputedError.Internal = fmt.Sprintf(
					"panic after response: %v -> response error: %s\n%s",
					panicErr.Unwrap(),
					ctx.caputedError.Internal,
					panicErr.Stack(),
				)
			}
		}

		ctx.logHTTPPanic(ctx.getMetrics())
		return
	}

	if ctx.code == 0 {
		ctx.code = 200
	}

	if ctx.code >= 400 && !ctx.disableErrorCapture {
		if ctx.main != ctx {
			ctx.main.disableErrorCapture = true
		}
		ctx.serveError()
	}

	if ctx.disableLogging {
		return
	}
	if ctx.main != ctx {
		ctx.main.disableLogging = true
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
