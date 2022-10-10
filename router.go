package nix

import (
	"time"
)

type Router struct {
	servers   map[int]*Server
	cleanupF  func() error
	startTime time.Time
	Logger    *Logger
	TaskMgr   TaskManager
}
