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

	"github.com/nixpare/logger/v2"
)

// ServeText serves a string (as raw bytes) to the client
func (ctx *Context) String(s string) {
	ctx.Write([]byte(s))
}

func (ctx *Context) JSON(data []byte) {
	ctx.Header().Set("Content-Type", "application/json")
	ctx.Write(data)
}

func (ctx *Context) NewReverseProxy(dest string) (*httputil.ReverseProxy, error) {
	URL, err := url.Parse(dest)
	if err != nil {
		return nil, err
	}

	reverseProxy := &httputil.ReverseProxy {
		Rewrite: func(pr *httputil.ProxyRequest) {
			pr.SetURL(URL)
			pr.SetXForwarded()

			pr.Out.RequestURI = ctx.r.RequestURI
			
			var query string
            var queryMap map[string][]string = ctx.r.URL.Query()
			if len(queryMap) != 0 {
				first := true
				for key, values := range queryMap {
                    for _, value := range values {
                        if key == "domain" || key == "subdomain" {
                            continue
                        }
    
                        if (first) {
                            first = false
                        } else {
                            query += "&"
                        }
    
                        switch key {
                        case "proxy-domain":
                            key = "domain"
                        case "proxy-subdomain":
                            key = "subdomain"
                        }
    
                        query += key + "=" + value
                    }
				}
			}

			pr.Out.RequestURI += "?" + query
			pr.Out.URL.RawQuery = query
		},
		ErrorLog: log.New(logger.DefaultLogger, fmt.Sprintf("Proxy [%s] ", dest), 0),
		ModifyResponse: func(r *http.Response) error {
			if strings.Contains(r.Header.Get("Server"), "PareServer") {
				r.Header.Del("Server")
			}
			return nil
		},
	}

	return reverseProxy, nil
}

// ReverseProxy runs a reverse proxy to the provided url. Returns an error is the
// url could not be parsed or if an error has occurred during the connection
func (ctx *Context) ReverseProxy(dest string) error {
	reverseProxy, err := ctx.NewReverseProxy(dest)
	if err != nil {
		return err
	}

	var returnErr error
	returnErrM := new(sync.Mutex)

	reverseProxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
		returnErrM.Lock()
		defer returnErrM.Unlock()
		returnErr = err
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
