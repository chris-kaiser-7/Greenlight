package main

import (
	"net/http"
	"os/exec"
)

func (app *application) testVideoHandler(w http.ResponseWriter, r *http.Request) {
	//w.Header().Set("Content-Type", "video/mp4")
	//w.Header().Set("Transfer-Encoding", "chunked")

	app.logger.Info("testVideoHandler")
	videoPath := "internal/videos/sample_video.mp4"
	cmd := exec.Command("ffmpeg", "-i", videoPath, "-f", "mp4", "-movflags", "frag_keyframe+empty_moov", "pipe:1")
	app.rewriter.Writer = w
	cmd.Stdout = app.rewriter
	cmd.Run()
	app.logger.Info("finished testVideoHandler")
}

func (app *application) serveVideoHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "video/mp4")
	w.Header().Set("Transfer-Encoding", "chunked")

	app.logger.Info("serve")
	app.rewriter.Writer = w

	//app.videoStream.AddWriter(w)
	app.logger.Info("serve finished")
}

func (app *application) startVideoHandler(w http.ResponseWriter, r *http.Request) {
	//go app.videoStream.StartFFmpeg()

	app.logger.Info("start")
	videoPath := "internal/videos/sample_video.mp4"
	cmd := exec.Command("ffmpeg", "-i", videoPath, "-f", "mp4", "-movflags", "frag_keyframe+empty_moov", "pipe:1")
	cmd.Stdout = app.rewriter
	cmd.Run()
	app.logger.Info("start finished 1")
}

//mezaros 22ed ave
//walking tour
//upload video

//system for syching and delivering video

//slipt ffmpeg stream to all listeners. basicly a background go routine is runing transcoding the video and te handler taps into the stream.
