package nix

import (
	"time"
)

type Router struct {
	servers        map[int]*Server
	cleanupF       func() error
	startTime      time.Time
	Logger         Logger
	TaskMgr        *TaskManager
	exitC          chan struct{}
	offlineClients map[string]offlineClient
	isInternalConn func(remoteAddress string) bool
}

func NewRouter(logger Logger) *Router {
	r := &Router{
		servers:        make(map[int]*Server),
		startTime:      time.Now(),
		Logger:         logger,
		exitC:          make(chan struct{}),
		offlineClients: make(map[string]offlineClient),
	}

	r.newTaskManager()

	return r
}

func (r *Router) Start() {
	r.startTime = time.Now()

	go r.TaskMgr.start()

	r.TaskMgr.wait()

	for range r.exitC {
	}
}

func (r *Router) Wait() {
	for range r.exitC {
	}
}

func (r *Router) Stop() {
	r.TaskMgr.stop()
	close(r.exitC)
}
