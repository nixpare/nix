package middleware

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/nixpare/nix/utility"
)

var (
	DefaultExtensions = [...]string{ ".txt", ".html", ".css", ".js", ".json", ".webmanifest", ".xml" }
	ErrCacheDisabled = errors.New("cache disabled")
)

type Content interface {
	URI() string
	Name() string
	Info() (ContentInfo, error)
	Reader() (io.ReadSeekCloser, error)
}

type ContentInfo struct {
	Modtime time.Time
	Size    int
}

type Cache struct {
	dir      string
	storage  map[string]*cacheStorage
    ttl      time.Duration
    exts     []string
	mutex    *sync.RWMutex
	logger   *log.Logger
    disabled bool
}

func NewCache(logger *log.Logger, dir string, ttl time.Duration, extensions []string, contents ...Content) (*Cache, error) {
	if logger == nil {
		logger = log.Default()
	}
	
	abs, err := filepath.Abs(dir)
	if err == nil {
		dir = abs
	}

	c := &Cache{
		dir:     dir,
		storage: make(map[string]*cacheStorage),
        ttl:     ttl,
        exts:    extensions,
		mutex:   new(sync.RWMutex),
		logger:  logger,
	}

	for _, content := range contents {
		err := c.NewContent(content)
		if err != nil {
			return nil, err
		}
	}

	return c, nil
}

func (c *Cache) SetFileCacheTTL(ttl time.Duration) {
	c.ttl = ttl
}

func (c *Cache) EnableFileCache() {
    c.mutex.Lock()
    defer c.mutex.Unlock()

    if !c.disabled {
        return
    }
	c.disabled = false
}

func (c *Cache) DisableFileCache() {
	c.mutex.Lock()
    defer c.mutex.Unlock()

    if c.disabled {
        return
    }
    c.disabled = true

	for _, s := range c.storage {
		s.data = nil
	}
}

func (c *Cache) UpdateCache() error {
	c.mutex.RLock()
    defer c.mutex.RUnlock()

	if c.disabled {
        return fmt.Errorf("cache update error: %w", ErrCacheDisabled)
    }

	var errs []error
	for path, s := range c.storage {
		err := s.update()
		if err != nil {
			s.data = nil
			errs = append(errs, fmt.Errorf("update content \"%s\": %w", path, err))
		}
	}

	return fmt.Errorf("cache update error: %w", errors.Join(errs...))
}

func (c *Cache) UpdateContent(uri string) error {
	c.mutex.RLock()
    defer c.mutex.RUnlock()

	if c.disabled {
        return nil
    }

	s := c.storage[uri]
	if s == nil {
		return fmt.Errorf("content update error: \"%s\" not found", uri)
	}

	err := s.update()
	if err != nil {
		return fmt.Errorf("content update error for \"%s\": %w", uri, err)
	}

	return nil
}

func (c *Cache) DumpStatus() string {
	c.mutex.RLock()
	defer c.mutex.RUnlock()

	var sb strings.Builder
	
	sb.WriteString("Status: ")
	if c.disabled {
		sb.WriteString("Disabled")
	} else {
		sb.WriteString("Enabled")
	}

	sb.WriteString(" - # Content: ")
	sb.WriteString(fmt.Sprint(len(c.storage)))

	sb.WriteString(" - Size: ")
	var data []int
	for _, cs := range c.storage {
		data = append(data, len(cs.data))
	}
	sb.WriteString(utility.PrintBytes(data...))

	for uri, cs := range c.storage {
		sb.WriteString("\n   - \"")
		sb.WriteString(uri)
		sb.WriteString("\" -> Size: ")
		sb.WriteString(utility.PrintBytes(len(cs.data)))
		sb.WriteString(" - Last Modify: ")
		sb.WriteString(cs.info.Modtime.Format(time.DateTime))
		sb.WriteString(" - Expiration: ")
		sb.WriteString(cs.expiration.Format(time.DateTime))
	}

	return sb.String()
}

func (c *Cache) ServeStatic(w http.ResponseWriter, r *http.Request) {
	c.ServeContent(w, r, r.URL.Path)
}

func (c *Cache) ServeContent(w http.ResponseWriter, r *http.Request, uri string) {
	c.mutex.RLock()
	cs := c.storage[uri]
	c.mutex.RUnlock()

	if cs == nil {
		var ( staticPath string; skipped bool; err error )
		cs, staticPath, skipped, err = c.getStaticFile(uri)
		
		if skipped {
			http.ServeFile(w, r, filepath.Join(c.dir, staticPath))
			return
		}

		if err != nil {
			c.logger.Printf("error getting static file at \"%s\": %v\n", staticPath, err)
			http.Error(w, "404 not found", http.StatusNotFound)
			return
		}

		if cs == nil {
			http.Error(w, "404 not found", http.StatusNotFound)
			return
		}
	}

	if c.disabled {
		c.serveContentNoCache(w, r, cs.content)
		return
	}
	
	expiration := cs.expiration

	if cs.data == nil || expiration.Before(time.Now()) {
		err := cs.update()
		if err != nil {
			c.mutex.Lock()
			delete(c.storage, uri)
			c.mutex.Unlock()

			c.logger.Printf("error updating content at \"%s\": %v\n", uri, err)
			http.Error(w, "404 not found", http.StatusNotFound)
			return
		}
	}

	http.ServeContent(
        w, r,
        cs.content.Name(), cs.info.Modtime,
        cs.reader(),
    )
}

func (c *Cache) Handler() http.Handler {
	return http.HandlerFunc(c.ServeStatic)
}

func (c *Cache) serveContentNoCache(w http.ResponseWriter, r *http.Request, content Content) {
	info, err := content.Info()
	if err != nil {
		http.Error(w, "404 not found", http.StatusNotFound)
		return
	}

	reader, err := content.Reader()
	if err != nil {
		http.Error(w, "404 not found", http.StatusNotFound)
		return
	}

    http.ServeContent(w, r, content.Name(), info.Modtime, reader)
}

func (c *Cache) getStaticFile(path string) (*cacheStorage, string, bool, error) {
	uri := path

	if path == "/" {
		path = "/index.html"
	}

	ext := filepath.Ext(path)
	if ext == "" {
		ext = ".html"
		path += ext
	}

	var found bool
	for _, e := range c.exts {
		if e == ext {
			found = true
			break
		}
	}
	if !found {
		return nil, path, true, nil
	}

	content := NewCachedFile(uri, c.dir, path)
	cs, err := c.newContent(content)
	if err != nil {
		if !os.IsNotExist(err) {
			return nil, path, false, err
		} else {
			return nil, path, false, nil
		}
	}

	return cs, path, false, nil
}
