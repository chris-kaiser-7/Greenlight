package main

import (
	"fmt"
	"net/http"
)

func (app *application) serveStreamHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		SDP string `json:"sdp"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	//TODO: validate sdp

	localDescription, err := app.videoStream.AddClient(input.SDP)
	if err != nil {
		fmt.Println(err)
		app.serverErrorResponse(w, r, err)
	}
	env := envelope{"sdp": localDescription}

	err = app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		fmt.Println(err)
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) controlStreamHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		Ctrl string `json:"ctrl"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	var env envelope
	if input.Ctrl == "init" {
		go func() {
			app.videoStream.InitStream()
		}()
		env = envelope{"message": "init"}
	} else if input.Ctrl == "add1" {
		go func() {
			app.videoStream.AddToStream(0)
		}()
		env = envelope{"message": "added 0"}
	} else if input.Ctrl == "add2" {
		go func() {
			app.videoStream.AddToStream(1)
		}()
		env = envelope{"message": "added 1"}
	} else if input.Ctrl == "start" {
		go func() {
			app.videoStream.StartStream()
		}()
		env = envelope{"message": "The stream has been started"}
	} else if input.Ctrl == "pause" {
		go func() {
			app.videoStream.PauseStream()
		}()
		env = envelope{"message": "The stream has been paused"}
	} else {
		env = envelope{"message": "Command not found"}
	}

	err = app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
