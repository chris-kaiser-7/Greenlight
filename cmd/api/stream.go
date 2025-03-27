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
	fmt.Println("out")
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
	env := envelope{"sdp": localDescription}

	err = app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}

func (app *application) controlStreamHandler(w http.ResponseWriter, r *http.Request) {
	var input struct {
		string `json:"sdp"`
	}

	err := app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}

	go func() {
		app.videoStream.StartStream()
	}()

	env := envelope{"message": "The stream has started"}
	err = app.writeJSON(w, http.StatusOK, env, nil)
	if err != nil {
		app.serverErrorResponse(w, r, err)
	}
}
