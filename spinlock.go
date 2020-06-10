package crab

import (
	"runtime"
	"sync/atomic"
)

type Locker struct {
	lock uintptr
}

func (l *Locker) Lock() {
	for !atomic.CompareAndSwapUintptr(&l.lock, 0, 1) {
		runtime.Gosched()
	}
}

func (l *Locker) Unlock() {
	atomic.StoreUintptr(&l.lock, 0)
}
