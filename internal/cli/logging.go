package cli

import (
	"fmt"
	"io"
	"os"
	"time"
)

var verbose bool

func logVerbose(w io.Writer, format string, args ...interface{}) {
	if !verbose {
		return
	}
	if w == nil {
		w = os.Stderr
	}
	fmt.Fprintf(w, "[ytm] "+format+"\n", args...)
}

type spinner struct {
	writer  io.Writer
	message string
	stop    chan struct{}
	done    chan struct{}
	chars   []rune
}

func newSpinner(w io.Writer, message string) *spinner {
	if verbose || w == nil {
		return nil
	}
	return &spinner{
		writer:  w,
		message: message,
		stop:    make(chan struct{}),
		done:    make(chan struct{}),
		chars:   []rune{'|', '/', '-', '\\'},
	}
}

func (s *spinner) Start() {
	if s == nil {
		return
	}
	go func() {
		defer close(s.done)
		idx := 0
		fmt.Fprintf(s.writer, "%s ", s.message)
		for {
			select {
			case <-s.stop:
				fmt.Fprintf(s.writer, "\r%s ✓\n", s.message)
				return
			case <-time.After(120 * time.Millisecond):
				char := s.chars[idx%len(s.chars)]
				fmt.Fprintf(s.writer, "\r%s %c", s.message, char)
				idx++
			}
		}
	}()
}

func (s *spinner) Stop() {
	if s == nil {
		return
	}
	select {
	case <-s.done:
		return
	case s.stop <- struct{}{}:
		<-s.done
	}
}
