package nix

import (
	"html/template"
	"net/http"

	"github.com/nixpare/logger/v3"
	"github.com/nixpare/nix/middleware"
)

type Nix struct {
    opts []Option
}

func New(opts ...Option) *Nix {
    return &Nix{ opts: opts }
}

func (n *Nix) Handle(handler func(ctx *Context)) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
		ctx := newContext(w, r)
		defer contextPool.Put(ctx)

		for _, opt := range n.opts {
			opt(ctx)
		}

		serveContext(ctx, handler)
	}
}

func (n *Nix) Wrap(handler http.Handler) http.HandlerFunc {
	return n.Handle(func(ctx *Context) {
		handler.ServeHTTP(ctx, ctx.R())
	})
}

func (n *Nix) WrapFunc(handler http.HandlerFunc) http.HandlerFunc {
	return n.Handle(func(ctx *Context) {
		handler(ctx, ctx.R())
	})
}

func NewHandler(handler func(ctx *Context), opts ...Option) http.HandlerFunc {
	return New(opts...).Handle(handler)
}

func NewWrapper(handler http.Handler, opts ...Option) http.HandlerFunc {
	return New(opts...).Wrap(handler)
}

func NewWrapperFunc(handler http.HandlerFunc, opts ...Option) http.HandlerFunc {
	return New(opts...).Wrap(handler)
}

type Option func(ctx *Context)

func LoggerOption(l *logger.Logger) Option {
	return func(ctx *Context) {
		ctx.SetLogger(l)
	}
}

func ErrorTemplateOption(t *template.Template) Option {
	return func(ctx *Context) {
		ctx.SetErrorTemplate(t)
	}
}

func EnableLoggingOption() Option {
	return func(ctx *Context) {
		ctx.enableLogging = true
	}
}

func CustomHostLogOption(host string) Option {
	return func(ctx *Context) {
		ctx.SetCustomHostLog(host)
	}
}

func EnableErrorCaptureOption() Option {
	return func(ctx *Context) {
		ctx.enableErrorCapture = true
	}
}

func EnableRecoveryOption() Option {
	return func(ctx *Context) {
		ctx.enableRecovery = true
	}
}

func MainOption() Option {
	return func(ctx *Context) {
		setMain(ctx.r, ctx)
	}
}

func ConnectToMainOption() Option {
	return func(ctx *Context) {
		ctx.main = GetMain(ctx.r)
	}
}

func CookieManagerOption(cm *middleware.CookieManager) Option {
	return func(ctx *Context) {
		ctx.cookieManager = cm
	}
}

func CacheOption(cache *middleware.Cache) Option {
	return func(ctx *Context) {
		ctx.cache = cache
	}
}
