package genesis

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

type GenesisImportConfig struct {
	StreamGenesisImport bool
	GenesisStreamFile   string
}

const bufferSize = 100000

// Stream owns its source. Consumers must drain Lines and check Wait, or Close
// the stream when they stop early. Closing Lines alone does not imply success.
type Stream struct {
	Lines  <-chan string
	done   chan struct{}
	cancel context.CancelFunc
	err    error
}

func IngestGenesisFileLineByLine(filename string) *Stream {
	return IngestGenesisFileLineByLineContext(context.Background(), filename)
}

func IngestGenesisFileLineByLineContext(ctx context.Context, filename string) *Stream {
	file, err := os.Open(filepath.Clean(filename))
	if err != nil {
		lines := make(chan string)
		close(lines)
		done := make(chan struct{})
		close(done)
		return &Stream{Lines: lines, done: done, cancel: func() {}, err: fmt.Errorf("open genesis file %s: %w", filename, err)}
	}
	return NewStream(ctx, file)
}

// NewStream reads a source with the same transport and completion semantics as
// file ingestion. Cancellation closes the source to interrupt a blocked read.
func NewStream(ctx context.Context, source io.ReadCloser) *Stream {
	ctx, cancel := context.WithCancel(ctx)
	lines := make(chan string)
	s := &Stream{Lines: lines, done: make(chan struct{}), cancel: cancel}
	var closeOnce sync.Once
	var closeErr error
	closeSource := func() { closeOnce.Do(func() { closeErr = source.Close() }) }
	go func() {
		select {
		case <-ctx.Done():
			closeSource()
		case <-s.done:
		}
	}()
	go func() {
		defer cancel()
		defer close(s.done)
		defer close(lines)
		defer func() {
			closeSource()
			if err := ctx.Err(); err != nil {
				s.err = err
			} else if closeErr != nil {
				s.err = errors.Join(s.err, fmt.Errorf("close genesis stream: %w", closeErr))
			}
		}()
		reader := bufio.NewReaderSize(source, bufferSize)
		for {
			if err := ctx.Err(); err != nil {
				s.err = err
				return
			}
			line, err := reader.ReadString('\n')
			if len(line) > 0 {
				select {
				case lines <- strings.TrimSuffix(line, "\n"):
				case <-ctx.Done():
					s.err = ctx.Err()
					return
				}
			}
			if err != nil {
				if err != io.EOF {
					s.err = fmt.Errorf("read genesis stream: %w", err)
				}
				return
			}
		}
	}()
	return s
}

func (s *Stream) Wait() error {
	<-s.done
	return s.err
}

func (s *Stream) Close() error {
	s.cancel()
	return s.Wait()
}

// Consume reports success only after the producer finishes successfully.
func (s *Stream) Consume(consume func(string) error) error {
	defer s.Close()
	for line := range s.Lines {
		if err := consume(line); err != nil {
			return err
		}
	}
	return s.Wait()
}
