package middleware

import (
	"errors"
	"io"
	"time"

	"github.com/nixpare/broadcaster"
)

type cacheStorage struct {
	cache      *Cache
	data       []byte
	content    Content
	info       ContentInfo
	expiration time.Time
	bc         *broadcaster.Broadcaster[struct{}]
}

func (c *Cache) NewContent(content Content) error {
	_, err := c.newContent(content)
	return err
}

func (c *Cache) newContent(content Content) (*cacheStorage, error) {
	cs := &cacheStorage{
		cache: c,
		content: content,
		bc: broadcaster.NewBroadcaster[struct{}](),
	}

	err := cs.update()
	if err != nil {
		return nil, err
	}

	c.mutex.Lock()
	c.storage[content.URI()] = cs
	c.mutex.Unlock()

	return cs, nil
}

func (s *cacheStorage) update() error {
	s.expiration = time.Now().Add(s.cache.ttl)

	info, err := s.content.Info()
	if err != nil {
		return err
	}
	if info.Modtime.Compare(s.info.Modtime) <= 0 {
		return nil
	}

	reader, err := s.content.Reader()
	if err != nil {
		return err
	}
	
	s.info = info
	if s.data == nil {
		s.data = make([]byte, 0, info.Size)
	} else {
		s.data = s.data[:0]
	}

	go func() {
		defer reader.Close()
		_, _ = io.Copy(s, reader)
	}()

	return nil
}

func (s *cacheStorage) length() int {
	return len(s.data)
}

func (s *cacheStorage) reader() io.ReadSeeker {
	return &cacheReader{ cs: s }
}

func (s *cacheStorage) Write(b []byte) (n int, err error) {
	if len(b) == 0 {
		return 0, nil
	}

	if len(b) > s.info.Size - s.length() {
		n = s.info.Size - s.length()
		err = errors.New("virtual file error: exeeded file size")
	} else {
		n = len(b)
	}

	s.data = append(s.data, b[:n]...)
	s.bc.Send(struct{}{})

	return
}

type cacheReader struct {
	cs *cacheStorage
	offset int64
}

// Read is used to implement the io.Reader interface
func (r *cacheReader) Read(p []byte) (n int, err error) {
	if len(p) == 0 {
		return 0, nil // Reading no data
	}

	if int(r.offset) == r.cs.info.Size {
		return 0, io.EOF // Charet position already off
	}

	var ch *broadcaster.Channel[struct{}]
	for len(p) > r.cs.length() - int(r.offset) && r.cs.length() < r.cs.info.Size {
		if ch == nil {
			ch = r.cs.bc.Register(20)
			defer ch.Unregister()
		}

		_, ok := <- ch.Ch()
		if !ok {
			break
		}
	}

	n = copy(p, r.cs.data[r.offset:])
	r.offset += int64(n)
	if int(r.offset) == r.cs.info.Size {
		err = io.EOF
	}

	return
}

// Seek is used to implement the io.Seeker interface
func (r *cacheReader) Seek(offset int64, whence int) (int64, error) {
	switch whence {
	case io.SeekStart:
		r.offset = offset
	case io.SeekCurrent:
		r.offset = int64(r.offset) + offset
	case io.SeekEnd:
		r.offset = int64(r.cs.info.Size) + offset
	default:
		return 0, errors.New("virtual file seek: invalid whence")
	}

	if r.offset < 0 {
		return 0, errors.New("virtual file seek: negative position")
	}
	return r.offset, nil
}
