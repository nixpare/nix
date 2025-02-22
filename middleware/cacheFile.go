package middleware

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

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
		err = fmt.Errorf("stat: %w", err)
		return
	}

	cInfo = ContentInfo{ Modtime: info.ModTime(), Size: int(info.Size()) }
	return
}

func (f cachedFile) Reader() (io.ReadSeekCloser, error) {
	file, err := os.OpenFile(f.path, os.O_RDONLY, 0)
	if err != nil {
		return nil, err
	}
	
	return file, nil
}
