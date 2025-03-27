package data

import (
	"fmt"
	"io"
	"os/exec"
	"sync"
	"time"
)

type StreamSplitter struct {
	mu      sync.Mutex
	writers []io.Writer
	Count   int
	Cmd     *exec.Cmd
}

func (s *StreamSplitter) AddWriter(w io.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.writers = append(s.writers, w)
	fmt.Println("added writer: ", w)
	s.Count += 1
}

func (s *StreamSplitter) Write(p []byte) (n int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, w := range s.writers {
		fmt.Println("writting to writer: ", i)
		_, err := w.Write(p)
		if err != nil {
			fmt.Println("Error writing to stream:", err)
		}
	}
	return len(p), nil
}
func (s *StreamSplitter) StartFFmpeg() *exec.Cmd {
	fmt.Println("starting FFmpeg")
	videoPath := "internal/videos/sample.mp4"
	cmd := exec.Command("ffmpeg", "-i", videoPath, "-f", "mp4", "-movflags", "frag_keyframe+empty_moov", "pipe:1")
	cmd.Stdout = s
	s.Cmd = cmd

	for {
		time.Sleep(time.Minute)
	}
}
