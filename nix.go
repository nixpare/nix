package nix

import (
	"html/template"
	"net/http"
	"time"

	"github.com/nixpare/logger/v2"
)

type Nix struct {
    opts []Option
}

func New(opts ...Option) *Nix {
    return &Nix{ opts: opts }
}

func (n *Nix) Handle(handler func(ctx *Context)) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
		ctx := &Context{
			w:   w,
			r:   r,
			connTime:            time.Now(),
		}

		main := GetMain(r)
		if main == nil {
			main = ctx
			setMain(r, ctx)
		} else {
			ctx.customHostLog = main.customHostLog
			ctx.disableLogging = main.disableLogging
			ctx.disableErrorCapture = main.disableErrorCapture
			ctx.errTemplate = main.errTemplate
		}
		ctx.main = main

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

func ConnectMain(handler func(ctx *Context)) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		main := GetMain(r)
		if main == nil {
			return
		}

		handler(main)
	}
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

func LoggerOption(l logger.Logger) Option {
	return func(ctx *Context) {
		ctx.l = l
	}
}

func ErrTemplateOption(templ *template.Template) Option {
	return func(ctx *Context) {
		ctx.errTemplate = templ
	}
}

func DisableLoggingOption() Option {
	return func(ctx *Context) {
		ctx.disableLogging = true
	}
}

func CustomHostLogOption(host string) Option {
	return func(ctx *Context) {
		ctx.customHostLog = host
	}
}

func DisableErrorCaptureOption() Option {
	return func(ctx *Context) {
		ctx.disableErrorCapture = true
	}
}
