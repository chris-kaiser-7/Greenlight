package main

import (
	"errors"
	"fmt"
	"github.com/julienschmidt/httprouter"
	"net/http"
	"strconv"
)

func readIdParam(r *http.Request, limit int64) (int64, error) {
	params := httprouter.ParamsFromContext(r.Context())

	id, err := strconv.ParseInt(params.ByName("id"), 10, 64)

	if err != nil || id < 0 || id >= limit {
		return 0, errors.New("invalid id parameter")
	}

	return id, nil
}

func (app *application) serveStreamHandler(w http.ResponseWriter, r *http.Request) {
	id, err := readIdParam(r, int64(len(app.videoStreams)))
	if err != nil {
		app.notFoundResponse(w, r)
		return
	}

	var input struct {
		SDP string `json:"sdp"`
	}

	err = app.readJSON(w, r, &input)
	if err != nil {
		app.badRequestResponse(w, r, err)
		return
	}
	//TODO: validate sdp

	fmt.Println("serving id:", id)
	localDescription, err := app.videoStreams[id].AddClient(input.SDP)
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
	if input.Ctrl == "start" {
		go func() {
			for _, stream := range app.videoStreams {
				stream.StartStream()
			}
		}()
		env = envelope{"message": "The stream has been started"}
	} else if input.Ctrl == "pause" {
		go func() {
			for _, stream := range app.videoStreams {
				stream.PauseStream()
			}
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
