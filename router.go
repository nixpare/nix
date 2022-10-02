package nix

import (
	"os"
	"sync"
	"time"
)

type Router struct {
	servers   map[int]*Server
	cleanupF  func() error
	startTime time.Time
	logFile *os.File
	fileMutexMap map[string]*sync.Mutex
	// offlineClients map[string]offlineClient
	// isInternalConn func(remoteAddress string) bool
	TaskMgr TaskManager
}