package data

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"slices"
	"time"

	// "github.com/pion/interceptor"
	// "github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/ivfreader"
)

const (
	videoFileName1 = "static/output1.ivf"
	videoFileName2 = "static/output3.ivf"
)

type Streamer struct {
	clients         []SDPClient
	clientStream    chan SDPClient
	delClientStream chan uint
	clientCounter   uint //TODO: user a better id

	PeerConnectionConfig webrtc.Configuration
	readers              []ivfReader
	rIndex               int
	outTrack             *webrtc.TrackLocalStaticSample
	outFrames            chan []byte
	ticker               *time.Ticker //TODO: verify that a single ticker can work
	ready                chan struct{}
	closeStreamer        chan struct{}

	// peerConnection       *webrtc.PeerConnection
	// streamReader         *DynamicReader
	mode uint8 //0 paused, 1 playing
}

type ivfReader struct {
	filename string
	reader   *ivfreader.IVFReader
	header   *ivfreader.IVFFileHeader
}

type SDPClient struct {
	peerConnection   *webrtc.PeerConnection
	SDP              string
	LocalDescription string
	id               uint
}

func (s *Streamer) InitStream() error {
	//TODO: move to factory function

	//if at end of tracks wait for new track to be added
	//set up call back or whatever to handle transitioning the track

	// Asynchronously take all packets in the channel and write them out to our
	// track

	s.closeStreamer = make(chan struct{})
	s.delClientStream = make(chan uint)
	s.clientStream = make(chan SDPClient)

	// routine for adding clients. routine cloes when s.closeStreamer closes
	go func() {
		//TODO: move this to a seperate type?
		for {
			select {
			case newClient := <-s.clientStream:
				s.clients = append(s.clients, newClient)
				fmt.Println("adding client client len: ", len(s.clients))

			case <-s.closeStreamer:
				return
			}
		}
	}()

	// routine for adding deleting clients. routine cloes when s.closeStreamer closes
	go func() {
		for {
			select {
			case id := <-s.delClientStream:
				//should swich to hashmap if client are expacted to be >1000 and prod can handle. slices are typicly better for len < 1000
				//I don't think prod server can handle >1000 clients atm. TODO: load performance testing on this

				fmt.Println("deleting id: ", id)
				i := slices.IndexFunc(s.clients, func(c SDPClient) bool {
					return c.id == id
				})
				if i == -1 {
					continue
				}
				fmt.Println("i: ", i)
				s.clients[i].peerConnection.Close()
				s.clients[i].peerConnection = nil
				s.clients = slices.Delete(s.clients, i, i+1)
				// s.clients = slices.DeleteFunc(s.clients, func(c SDPClient) bool {
				// 	return c.id == id
				// })
				fmt.Println("deleting client client len: ", len(s.clients))

			case <-s.closeStreamer:
				return
			}
		}
	}()

	var videoTrackErr error
	s.outTrack, videoTrackErr = webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "pion",
	)
	if videoTrackErr != nil {
		panic(videoTrackErr)
	}

	s.ready = make(chan struct{})
	s.outFrames = make(chan []byte)
	s.ticker = time.NewTicker(time.Second)
	go func() {
		for ; true; <-s.ticker.C {
			if s.mode == 1 {
				frame := <-s.outFrames
				if ivfErr := s.outTrack.WriteSample(media.Sample{Data: frame, Duration: time.Second}); ivfErr != nil {
					panic(ivfErr)
				}
			}
		}
	}()

	go func() {
		<-s.ready
		for {
			curReader := &s.readers[s.rIndex]
			frame, _, ivfErr := curReader.reader.ParseNextFrame()
			if errors.Is(ivfErr, io.EOF) {
				fmt.Println("eof next track")
				file, openErr := os.Open(curReader.filename)
				if openErr != nil {
					panic(openErr)
				}
				reader, header, openErr := ivfreader.NewWith(file)
				if openErr != nil {
					panic(openErr)
				}
				curReader.reader = reader
				curReader.header = header
				s.rIndex += 1
				if s.rIndex >= len(s.readers) {
					s.rIndex = 0
				}
				curReader = &s.readers[s.rIndex]
				s.ticker = time.NewTicker(
					time.Millisecond * time.Duration((float32(curReader.header.TimebaseNumerator)/
						float32(curReader.header.TimebaseDenominator))*1000))

			} else if ivfErr != nil {
				panic(ivfErr)
			} else {
				s.outFrames <- frame
			}
		}
	}()

	return nil
}

func (s *Streamer) AddToStream(n uint8) error {
	//validation
	var videoFileName string
	if n == 0 {
		videoFileName = videoFileName1
	} else {
		videoFileName = videoFileName2
	}
	_, err := os.Stat(videoFileName)
	haveVideoFile := !errors.Is(err, fs.ErrNotExist)
	if !haveVideoFile {
		panic("Could not find `" + videoFileName + "`")
	}

	file, openErr := os.Open(videoFileName)
	if openErr != nil {
		panic(openErr)
	}

	reader, header, openErr := ivfreader.NewWith(file)
	if openErr != nil {
		panic(openErr)
	}

	s.readers = append(s.readers, ivfReader{reader: reader, header: header, filename: videoFileName})

	if len(s.readers) == 1 {
		close(s.ready)
		curReader := &s.readers[0]
		s.ticker = time.NewTicker(
			time.Millisecond * time.Duration(
				(float32(curReader.header.TimebaseNumerator)/
					float32(curReader.header.TimebaseDenominator))*1000))
	}

	return nil
}

func (s *Streamer) StartStream() error {
	fmt.Println("starting stream")
	s.mode = 1

	return nil
}

func (s *Streamer) PauseStream() error {
	fmt.Println("pausing stream")
	s.mode = 0

	return nil
}

func (s *Streamer) AddClient(sdp string) (string, error) {
	recvOnlyOffer := webrtc.SessionDescription{}
	decode(sdp, &recvOnlyOffer)

	//s.clients = append(s.clients, SDPClient{SDP: sdp})
	//newClient := &s.clients[len(s.clients)-1]
	newClient := SDPClient{id: s.clientCounter}
	s.clientCounter += 1

	// Create a new PeerConnection
	var err error
	newClient.peerConnection, err = webrtc.NewPeerConnection(s.PeerConnectionConfig)
	if err != nil {
		return "", err
	}

	_, iceConnectedCtxCancel := context.WithCancel(context.Background())

	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	newClient.peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
		fmt.Printf("Connection State has changed %s \n", connectionState.String())
		if connectionState == webrtc.ICEConnectionStateConnected {
			iceConnectedCtxCancel()
		}
	})

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	newClient.peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", state.String())

		if state == webrtc.PeerConnectionStateFailed {
			// Wait until PeerConnection has had no network activity for 30 seconds or another failure.
			// It may be reconnected using an ICE Restart.
			// Use webrtc.PeerConnectionStateDisconnected if you are interested in detecting faster timeout.
			// Note that the PeerConnection may come back from PeerConnectionStateDisconnected.
			fmt.Println("Peer Connection has gone to failed exiting")
			s.delClientStream <- newClient.id
		}

		if state == webrtc.PeerConnectionStateClosed {
			// PeerConnection was explicitly closed. This usually happens from a DTLS CloseNotify
			fmt.Println("Peer Connection has gone to closed exiting")
			s.delClientStream <- newClient.id
		}
	})

	rtpSender, err := newClient.peerConnection.AddTrack(s.outTrack)
	if err != nil {
		return "", err
	}

	readRTCP(rtpSender)

	// Set the remote SessionDescription
	err = newClient.peerConnection.SetRemoteDescription(recvOnlyOffer) //1
	if err != nil {
		return "", err
	}

	// Create answer
	answer, err := newClient.peerConnection.CreateAnswer(nil) //2
	if err != nil {
		return "", err
	}

	// Create channel that is blocked until ICE Gathering is complete
	gatherComplete := webrtc.GatheringCompletePromise(newClient.peerConnection) //3
	//<-iceConnectedCtx.Done()

	// Sets the LocalDescription, and starts our UDP listeners
	err = newClient.peerConnection.SetLocalDescription(answer) //4
	if err != nil {
		return "", err
	}

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	<-gatherComplete //5
	//<-iceConnectedCtx.Done()

	// Get the LocalDescription and take it to base64 so we can paste in browser
	newClient.LocalDescription = encode(newClient.peerConnection.LocalDescription())
	s.clientStream <- newClient
	fmt.Println("added to stream")

	return newClient.LocalDescription, nil
}

func readRTCP(rtpSender *webrtc.RTPSender) {
	// Read incoming RTCP packets
	// Before these packets are returned they are processed by interceptors. For things
	// like NACK this needs to be called.
	go func() {
		rtcpBuf := make([]byte, 1500)
		for {
			if _, _, rtcpErr := rtpSender.Read(rtcpBuf); rtcpErr != nil {
				return
			}
		}
	}()
}

// JSON encode + base64 a SessionDescription.
func encode(obj *webrtc.SessionDescription) string {
	b, err := json.Marshal(obj)
	if err != nil {
		panic(err)
	}

	return base64.StdEncoding.EncodeToString(b)
}

// Decode a base64 and unmarshal JSON into a SessionDescription.
func decode(in string, obj *webrtc.SessionDescription) {
	b, err := base64.StdEncoding.DecodeString(in)
	if err != nil {
		panic(err)
	}

	if err = json.Unmarshal(b, obj); err != nil {
		panic(err)
	}
}
