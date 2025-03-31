package data

import (
	"io"
	"sync"
)

type DynamicReader struct {
	mu      sync.Mutex
	readers []io.Reader
}

func NewDynamicReader() *DynamicReader {
	return &DynamicReader{
		readers: make([]io.Reader, 0),
	}
}

func (dr *DynamicReader) AddReader(r io.Reader) {
	dr.mu.Lock()
	defer dr.mu.Unlock()
	dr.readers = append(dr.readers, r)
}

func (dr *DynamicReader) Read(p []byte) (int, error) {
	for {
		dr.mu.Lock()
		if len(dr.readers) == 0 {
			dr.mu.Unlock()
			return 0, io.EOF
		}

		current := dr.readers[0]
		dr.mu.Unlock()

		n, err := current.Read(p)
		if err == io.EOF {
			// Remove exhausted reader
			dr.mu.Lock()
			dr.readers = dr.readers[1:]
			dr.mu.Unlock()
			if n > 0 {
				return n, nil
			}
			continue // Try next reader
		}
		return n, err
	}
}
