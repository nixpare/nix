package nix

import (
	"io"
	"time"
)

type Router struct {
	servers   map[int]*Server
	cleanupF  func() error
	startTime time.Time
	Logger    Logger
	TaskMgr   TaskManager
}

func NewRouter(out io.Writer) *Router {
	r := &Router{
		servers:   make(map[int]*Server),
		startTime: time.Now(),
		Logger:    NewLogger(out),
	}

	return r
}
