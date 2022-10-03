package nix

import (
	"os"
	"time"
)

type Router struct {
	servers   map[int]*Server
	cleanupF  func() error
	startTime time.Time
	logFile *os.File
	TaskMgr TaskManager
}