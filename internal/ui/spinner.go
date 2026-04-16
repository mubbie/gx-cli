package ui

import (
	"fmt"
	"sync"
	"time"
)

var spinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

// Spinner shows an animated indicator while work is in progress.
// Has a 200ms grace period before showing to avoid flashing on fast operations.
type Spinner struct {
	stop    chan struct{}
	done    sync.WaitGroup
	showed  bool
}

// StartSpinner begins a spinner that appears after 200ms.
func StartSpinner(msg string) *Spinner {
	s := &Spinner{stop: make(chan struct{})}
	s.done.Add(1)
	go func() {
		defer s.done.Done()

		// Grace period: don't show spinner for fast operations
		select {
		case <-s.stop:
			return
		case <-time.After(200 * time.Millisecond):
		}

		s.showed = true
		i := 0
		for {
			select {
			case <-s.stop:
				fmt.Printf("\r\033[K")
				return
			default:
				frame := spinnerFrames[i%len(spinnerFrames)]
				fmt.Printf("\r%s %s", BranchStyle.Render(frame), DimStyle.Render(msg))
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
	return s
}

// Stop halts the spinner and clears the line.
func (s *Spinner) Stop() {
	close(s.stop)
	s.done.Wait()
}
