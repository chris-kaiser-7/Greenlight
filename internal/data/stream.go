package data

import (
	"fmt"
	"io"
	"log"
	"sync"
)

// Stream structure to manage multiple clients
type Stream struct {
	mu      sync.Mutex
	clients []io.Writer
}

// Add a client to the stream
func (s *Stream) AddClient(w io.Writer) {
	fmt.Println("adding writter ")
	s.mu.Lock()
	defer s.mu.Unlock()
	s.clients = append(s.clients, w)
}

func (s *Stream) RemoveClient(w io.Writer) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i, client := range s.clients {
		if client == w {
			s.clients = append(s.clients[:i], s.clients[i+1:]...)
			break
		}
	}
}

func (s *Stream) Write(data []byte) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Iterate through clients and remove disconnected ones
	activeClients := s.clients[:0]
	for _, client := range s.clients {
		if _, err := client.Write(data); err != nil {
			log.Println("Client disconnected:", err)
		} else {
			activeClients = append(activeClients, client)
		}
	}
	s.clients = activeClients
}

// Start FFmpeg and send output to clients
func (s *Stream) StartFFmpeg() {

	//	videoPath := "internal/videos/sample.mp4"
	//	cmd := exec.Command("ffmpeg", "-i", videoPath, "-f", "mp4", "-movflags", "frag_keyframe+empty_moov", "pipe:1")

	/*cmd := exec.Command("ffmpeg",
		"-re",             // Read input at native frame rate
		"-i", "input.mp4", // Replace with your actual input source
		"-c:v", "libx264",
		"-preset", "fast",
		"-f", "mpegts",
		"-codec:v", "mpeg1video",
		"-b:v", "1000k",
		"-muxdelay", "0.1",
		"pipe:1",
	)*/

	/*

		// Create pipe for FFmpeg output
		ffmpegOut, err := cmd.StdoutPipe()
		if err != nil {
			log.Fatal("Failed to get FFmpeg output:", err)
		}

		// Start FFmpeg
		err = cmd.Start()
		if err != nil {
			log.Fatal("Failed to start FFmpeg:", err)
		}

		// Read FFmpeg output and broadcast to clients
		buf := make([]byte, 1024)
		for {
			n, err := ffmpegOut.Read(buf)
			if err != nil {
				if err == io.EOF {
					break
				}
				log.Println("Error reading from FFmpeg:", err)
				continue
			}
			s.Broadcast(buf[:n])
		}

		// Wait for FFmpeg to finish
		cmd.Wait() */
}
