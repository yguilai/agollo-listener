package apollox

import (
	"sync"
)

type RoutineGroup struct {
	wg sync.WaitGroup
}

func NewRoutineGroup() *RoutineGroup {
	return new(RoutineGroup)
}

func (g *RoutineGroup) Run(fn func()) {
	g.wg.Add(1)
	go func() {
		defer g.wg.Done()
		fn()
	}()
}

func (g *RoutineGroup) RunSafe(fn func()) {
	g.wg.Add(1)
	GoSafe(func() {
		defer g.wg.Done()
		fn()
	})
}

func (g *RoutineGroup) Wait() {
	g.wg.Wait()
}
