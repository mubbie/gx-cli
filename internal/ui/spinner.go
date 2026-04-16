package ui

import (
	"fmt"
	"sync"
	"time"
)

// Spinner frames from charmbracelet/bubbles dot spinner
var spinnerFrames = []string{"⣾", "⣽", "⣻", "⢿", "⡿", "⣟", "⣯", "⣷"}

type Spinner struct {
	stop chan struct{}
	done sync.WaitGroup
}

func StartSpinner(msg string) *Spinner {
	s := &Spinner{stop: make(chan struct{})}
	s.done.Add(1)
	go func() {
		defer s.done.Done()
		i := 0
		for {
			select {
			case <-s.stop:
				fmt.Printf("\r\033[K")
				return
			default:
				frame := spinnerFrames[i%len(spinnerFrames)]
				styled := BranchStyle.Render(frame)
				fmt.Printf("\r%s %s", styled, DimStyle.Render(msg))
				i++
				time.Sleep(80 * time.Millisecond)
			}
		}
	}()
	return s
}

func (s *Spinner) Stop() {
	close(s.stop)
	s.done.Wait()
}
