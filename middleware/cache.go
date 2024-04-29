package middleware

import (
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
	DefaultExtensions = [...]string{ ".txt", ".html", ".css", ".js", ".json" }
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
	mutex    sync.RWMutex
	logger   *log.Logger
    disabled bool
}

func NewCache(logger *log.Logger, dir string, ttl time.Duration, extensions []string, contents ...Content) *Cache {
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
		logger:  logger,
	}

	for _, content := range contents {
		c.NewContent(content)
	}

	return c
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

func (c *Cache) UpdateCache() {
	c.mutex.RLock()
    defer c.mutex.RUnlock()

	for path, s := range c.storage {
		err := s.update()
		if err != nil {
			c.logger.Printf("error updating content at \"%s\": %v\n", path, err)
		}
	}
}

func (c *Cache) UpdateContent(uri string) error {
	c.mutex.RLock()
    defer c.mutex.RUnlock()

	s := c.storage[uri]
	if s == nil {
		return fmt.Errorf("content with path \"%s\" not found", uri)
	}

	err := s.update()
	if err != nil {
		return fmt.Errorf("error updating content at \"%s\": %v", uri, err)
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
		sb.WriteString("\" - Last Modify: ")
		sb.WriteString(cs.info.Modtime.Format(time.DateTime))
		sb.WriteString("\" - Expiration: ")
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
		var ( staticPath string; skipped bool )
		cs, staticPath, skipped = c.getStaticFile(uri)
		
		if skipped {
			http.ServeFile(w, r, filepath.Join(c.dir, staticPath))
			return
		}
	}

	if c.disabled {
		c.serveContentNoCache(w, r, cs.content)
		return
	}
	
	cs.mutex.RLock()
	expiration := cs.expiration
	cs.mutex.RUnlock()

	if expiration.Before(time.Now()) {
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

func (c *Cache) getStaticFile(path string) (*cacheStorage, string, bool) {
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
		return nil, path, true
	}

	content := NewCachedFile(uri, c.dir, path)
	return c.newContent(content), path, false
}

type cachedFile struct {
	uri string
	path string
}

func NewCachedFile(uri string, dir string, path string) Content {
	if path == "/" {
		path = "."
	} else {
		path = strings.TrimPrefix(path, "/")
	}

	path = filepath.Join(dir, path)

    return cachedFile{
		uri: uri,
		path: path,
	}
}

func (f cachedFile) URI() string {
	return f.uri
}

func (f cachedFile) Name() string {
	return filepath.Base(f.path)
}

func (f cachedFile) Info() (cInfo ContentInfo, err error) {
	info, err := os.Stat(f.path)
	if err != nil {
		return
	}

	cInfo = ContentInfo{ Modtime: info.ModTime(), Size: int(info.Size()) }
	return
}

func (f cachedFile) Reader() (io.ReadSeekCloser, error) {
	file, err := os.Open(f.path)
	if err != nil {
		return nil, err
	}
	
	return file, nil
}
