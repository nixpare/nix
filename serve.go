package nix

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/http/httputil"
	"net/url"
	"strings"
	"sync"

	"github.com/koding/websocketproxy"
)

// ServeText serves a string (as raw bytes) to the client
func (ctx *Context) String(s string) {
	ctx.Write([]byte(s))
}

func (ctx *Context) JSON(data []byte) {
	ctx.Header().Set("Content-Type", "application/json")
	ctx.Write(data)
}

func (ctx *Context) MimeType(mime string) {
	ctx.Header().Set("Content-Type", mime)
}

func (ctx *Context) NewReverseProxy(dest string) (http.Handler, *httputil.ReverseProxy, *websocketproxy.WebsocketProxy, error) {
	URL, err := url.Parse(dest)
	if err != nil {
		return nil, nil, nil, err
	}

	httpProxy := httputil.NewSingleHostReverseProxy(URL)
	httpProxy.ErrorLog = log.New(ctx.Logger(), fmt.Sprintf("Proxy [%v -> %s] ", ctx.r.URL, dest), 0)
	httpProxy.ModifyResponse = func(r *http.Response) error {
		if strings.Contains(r.Header.Get("Server"), "PareServer") {
			r.Header.Del("Server")
		}
		return nil
	}

	wsURL := new(url.URL)
	*wsURL = *URL
	wsURL.Scheme = "ws"

    wsProxy := websocketproxy.NewProxy(wsURL)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				ctx.code = http.StatusBadGateway
				ctx.AddInteralMessage(err)
			}
		}()

        if ctx.IsWebSocketRequest() {
            wsProxy.ServeHTTP(w, r)
        } else {
            httpProxy.ServeHTTP(w, r)
        }
    }), httpProxy, wsProxy, nil
}

func (ctx *Context) IsWebSocketRequest() bool {
    return IsWebSocketRequest(ctx.r)
}

func IsWebSocketRequest(r *http.Request) bool {
    connectionHeader := strings.ToLower(r.Header.Get("Connection"))
    upgradeHeader := strings.ToLower(r.Header.Get("Upgrade"))
	
    return (r.Method == http.MethodGet || r.Method == http.MethodConnect) &&
		strings.Contains(connectionHeader, "upgrade") &&
		upgradeHeader == "websocket"
}

// ReverseProxy runs a reverse proxy to the provided url. Returns an error is the
// url could not be parsed or if an error has occurred during the connection
func (ctx *Context) ReverseProxy(dest string) error {
	reverseProxy, httpProxy, _, err := ctx.NewReverseProxy(dest)
	if err != nil {
		return err
	}

	var returnErr error
	returnErrM := new(sync.Mutex)

	httpProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		returnErrM.Lock()
		defer returnErrM.Unlock()
		returnErr = fmt.Errorf("http reverse proxy error: %w", err)
	}

	reverseProxy.ServeHTTP(ctx, ctx.r)

	returnErrM.Lock()
	defer returnErrM.Unlock()
	return returnErr
}

// Body returns the response body bytes
func (ctx *Context) Body() ([]byte, error) {
	return io.ReadAll(ctx.r.Body)
}

// BodyString returns the response body as a string
func (ctx *Context) BodyString() (string, error) {
	data, err := ctx.Body()
	return string(data), err
}

func (ctx *Context) ReadJSON(value any) error {
	if ctype := ctx.R().Header.Get("Content-Type"); ctype != "application/json" {
        return fmt.Errorf("invalid content-type: found %s", ctype)
    }
    
    data, err := ctx.Body()
	if err != nil {
		return err
	}

	return json.Unmarshal(data, value)
}
