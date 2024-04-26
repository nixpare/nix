package nix

import (
	"bytes"
	"embed"
	"fmt"
	"html/template"
	"net/http"
	"strings"

	"github.com/nixpare/logger/v2"
)

//go:embed static
var staticFS embed.FS

func DefaultErrTemplate() *template.Template {
	data, err := staticFS.ReadFile("static/error.html")
	if err != nil {
		panic(err)
	}

	templ, err := template.New("error.html").Parse(string(data))
	if err != nil {
		panic(err)
	}

	return templ
}

type CapturedError struct {
	Code     int
	Data     []byte
	Internal string
}

func (err CapturedError) Message() string {
	return string(err.Data)
}

func (err CapturedError) Error() string {
	return fmt.Sprintf(`{"code": %d, "message": "%s", "internal": "%s"}`, err.Code, err.Data, err.Internal)
}

func (ctx *Context) writeError(data []byte, ctype string) {
	ctx.w.Header().Set("Content-Type", ctype)
	ctx.w.WriteHeader(ctx.code)
	ctx.w.Write(data)
}

// serveError serves the error in a predefines error template (if set) and only
// if no other information was alredy sent to the ResponseWriter. If there is no
// error template or if the connection method is different from GET or HEAD, the
// error message is sent as a plain text
func (ctx *Context) serveError() {
	ctype := http.DetectContentType(ctx.caputedError.Data)
	if len(ctx.caputedError.Data) != 0 {
		if strings.Contains(ctype, "text/html") {
			ctx.writeError(ctx.caputedError.Data, ctype)
			return
		}
	}

	if len(ctx.caputedError.Data) == 0 {
		ctx.writeError(ctx.caputedError.Data, ctype)
		return
	}

	if ctx.errTemplate == nil {
		ctx.writeError(ctx.caputedError.Data, ctype)
		return
	}

	if ctx.r.Method != "GET" && ctx.r.Method != "HEAD" {
		ctx.writeError(ctx.caputedError.Data, ctype)
		return
	}

	b := bytes.NewBuffer(nil)
	if err := ctx.errTemplate.Execute(b, ctx.caputedError); err != nil {
		ctx.l.Printf(logger.LOG_LEVEL_ERROR, "Error serving template file: %v", err)
		ctx.writeError(ctx.caputedError.Data, ctype)
		return
	}

	ctype = http.DetectContentType(b.Bytes())
	ctx.writeError(b.Bytes(), ctype)
}

// Error is used to manually report an HTTP Error to send to the
// client.
//
// It sets the http status code (so it should not be set
// before) and if the connection is done via a GET request, it will
// try to serve the html Error template with the status code and
// Error message in it, otherwise if the Error template does not exist
// or the request is done via another method (like POST), the Error
// message will be sent as a plain text.
//
// The last optional list of elements can be used just for logging or
// debugging: the elements will be saved in the logs
func (ctx *Context) Error(statusCode int, message string, a ...any) {
	ctx.WriteHeader(statusCode)

	if message == "" {
		message = "Unknown error"
	}

	ctx.Write([]byte(message))

	first := true
	for _, x := range a {
		if first {
			first = false
		} else {
			ctx.caputedError.Internal += " "
		}

		ctx.caputedError.Internal += fmt.Sprint(x)
	}
}
