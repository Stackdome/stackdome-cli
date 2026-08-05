package output

import (
	"fmt"
	"os"
	"sync"
	"time"
)

var frames = []string{"⠋", "⠙", "⠹", "⠸", "⠼", "⠴", "⠦", "⠧", "⠇", "⠏"}

type Spinner struct {
	message  string
	stop     chan struct{}
	done     sync.WaitGroup
	stopOnce sync.Once
}

func NewSpinner(message string) *Spinner {
	return &Spinner{
		message: message,
		stop:    make(chan struct{}),
	}
}

func (s *Spinner) Start() {
	if !IsTTY() {
		fmt.Fprintf(os.Stderr, "%s\n", s.message)
		return
	}
	s.done.Add(1)
	go func() {
		defer s.done.Done()
		i := 0
		tick := time.NewTicker(80 * time.Millisecond)
		defer tick.Stop()
		for {
			select {
			case <-s.stop:
				fmt.Fprintf(os.Stderr, "\r\033[K")
				return
			case <-tick.C:
				fmt.Fprintf(os.Stderr, "\r%s %s", frames[i%len(frames)], s.message)
				i++
			}
		}
	}()
}

func (s *Spinner) Stop() {
	s.stopOnce.Do(func() { close(s.stop) })
	s.done.Wait()
}
