package nix

import "net/http"

func (ctx *Context) Method() string {
	return ctx.r.Method
}

func (ctx *Context) RequestHeader() http.Header {
	return ctx.r.Header
}

func (ctx *Context) RequestPath() string {
	return ctx.r.URL.Path
}
