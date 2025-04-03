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

func (ctx *Context) RemoteAddr() string {
	if ctx.main != nil {
		return ctx.main.RemoteAddr()
	}

	return ctx.remoteAddr
}

func (ctx *Context) SetRemoteAddr(addr string) {
	ctx.remoteAddr = addr

	if ctx.main != nil {
		ctx.main.SetRemoteAddr(addr)
	}
}
