package nix

import (
	"io"
	"io/fs"
	"net/http"
	"time"

	"github.com/nixpare/nix/middleware"
)

func (ctx *Context) ServeContent(name string, modtime time.Time, content io.ReadSeeker) {
	http.ServeContent(ctx, ctx.r, name, modtime, content)
}

func (ctx *Context) ServeFile(name string) {
	http.ServeFile(ctx, ctx.r, name)
}

func (ctx *Context) ServeFileFS(dir fs.FS, name string) {
	http.ServeFileFS(ctx, ctx.r, dir, name)
}

func (ctx *Context) ServeCachedContent(uri string) {
	ctx.cache.ServeStatic(ctx, ctx.r)
}

func (ctx *Context) ServeStatic() {
	ctx.cache.ServeStatic(ctx, ctx.r)
}

func (ctx *Context) SetCookie(name string, value any, maxAge int, opts ...middleware.CookieOption) error {
	return ctx.cookieManager.SetCookie(ctx, name, value, maxAge, opts...)
}

func (ctx *Context) GetCookie(name string, value any) error {
	return ctx.cookieManager.GetCookie(ctx.r, name, value)
}

func (ctx *Context) DeleteCookie(name string) {
	ctx.cookieManager.DeleteCookie(ctx, name)
}

func (ctx *Context) SetCookiePerm(name string, value any, maxAge int, opts ...middleware.CookieOption) error {
	return ctx.cookieManager.SetCookiePerm(ctx, name, value, maxAge, opts...)
}

func (ctx *Context) GetCookiePerm(name string, value any) error {
	return ctx.cookieManager.GetCookiePerm(ctx.r, name, value)
}

func (ctx *Context) Redirect(url string, code int) {
	http.Redirect(ctx, ctx.r, url, code)
}