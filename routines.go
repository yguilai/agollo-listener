package agollolistener

import (
	"sync"
	"time"
)

func GoSafe(fn func()) {
	go RunSafe(fn)
}

func RunSafe(fn func()) {
	defer func() {
		if p := recover(); p != nil {
			getLogger().Errorf("got a panic: %v", p)
		}
	}()

	fn()
}

func WaitWithTimeout(wg *sync.WaitGroup, timeout time.Duration) bool {
	done := make(chan struct{}, 1)

	go func() {
		wg.Wait()
		close(done)
	}()

	timer := time.NewTimer(timeout)
	defer timer.Stop()

	select {
	case <-done:
		return true
	case <-timer.C:
		return false
	}
}
