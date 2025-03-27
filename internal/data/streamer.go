package data

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"time"

	"github.com/pion/interceptor"
	"github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/ivfreader"
)

const (
	videoFileName = "internal/videos/output.ivf"
)

type Streamer struct {
	clients              []SDPClient
	peerConnectionConfig webrtc.Configuration
	peerConnection       *webrtc.PeerConnection
	track                *webrtc.TrackLocalStaticSample
}

type Track struct {
	sample *webrtc.TrackLocalStaticSample
	mode   uint8
	ticker time.Ticker
}

type SDPClient struct {
	SDP              string
	LocalDescription string
	peerConnection   *webrtc.PeerConnection
}

func (s *Streamer) InitStream() error {

	return nil
}

// starts the stream. blocks untill error
func (s *Streamer) StartStream() error {
	// Assert that we have an audio or video file
	_, err := os.Stat(videoFileName)
	haveVideoFile := !errors.Is(err, fs.ErrNotExist)

	if !haveVideoFile {
		panic("Could not find `" + videoFileName + "`")
	}

	s.peerConnectionConfig = webrtc.Configuration{
		ICEServers: []webrtc.ICEServer{
			{
				URLs: []string{"stun:stun.l.google.com:19302"},
			},
		},
	}

	mediaEngine := &webrtc.MediaEngine{}
	if err := mediaEngine.RegisterDefaultCodecs(); err != nil {
		panic(err)
	}

	interceptorRegistry := &interceptor.Registry{}

	if err := webrtc.RegisterDefaultInterceptors(mediaEngine, interceptorRegistry); err != nil {
		panic(err)
	}

	intervalPliFactory, err := intervalpli.NewReceiverInterceptor()
	if err != nil {
		panic(err)
	}
	interceptorRegistry.Add(intervalPliFactory)

	s.peerConnection, err = webrtc.NewAPI(
		webrtc.WithMediaEngine(mediaEngine),
		webrtc.WithInterceptorRegistry(interceptorRegistry),
	).NewPeerConnection(s.peerConnectionConfig)
	if err != nil {
		panic(err)
	}
	defer func() {
		if cErr := s.peerConnection.Close(); cErr != nil {
			fmt.Printf("cannot close peerConnection: %v\n", cErr)
		}
	}()

	file, openErr := os.Open(videoFileName)
	if openErr != nil {
		panic(openErr)
	}

	ivf, header, openErr := ivfreader.NewWith(file)
	if openErr != nil {
		panic(openErr)
	}

	var trackCodec string
	switch header.FourCC {
	case "AV01":
		trackCodec = webrtc.MimeTypeAV1
	case "VP90":
		trackCodec = webrtc.MimeTypeVP9
	case "VP80":
		trackCodec = webrtc.MimeTypeVP8
	default:
		panic(fmt.Sprintf("Unable to handle FourCC %s", header.FourCC))
	}

	// Create a video track
	var videoTrackErr error
	s.track, videoTrackErr = webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: trackCodec}, "video", "pion",
	)
	if videoTrackErr != nil {
		panic(videoTrackErr)
	}

	rtpSender, videoTrackErr := s.peerConnection.AddTrack(s.track)
	if videoTrackErr != nil {
		panic(videoTrackErr)
	}

	readRTCP(rtpSender)

	fmt.Println("starting routine")
	go func() {
		// TODO: test if this is ok to be commented out
		//
		// file, ivfErr := os.Open(videoFileName)
		// if ivfErr != nil {
		// 	panic(ivfErr)
		// }
		//
		// ivf, header, ivfErr := ivfreader.NewWith(file)
		// if ivfErr != nil {
		// 	panic(ivfErr)
		// }

		//fmt.Println("waiting on ice")
		//<-iceConnectedCtx.Done()

		ticker := time.NewTicker(
			time.Millisecond * time.Duration((float32(header.TimebaseNumerator)/float32(header.TimebaseDenominator))*1000), // LOOKAT
		)
		defer ticker.Stop()
		for ; true; <-ticker.C {
			frame, _, ivfErr := ivf.ParseNextFrame()
			if errors.Is(ivfErr, io.EOF) {
				fmt.Printf("All video frames parsed and sent")
				os.Exit(0)
			}

			if ivfErr != nil {
				panic(ivfErr)
			}

			if ivfErr = s.track.WriteSample(media.Sample{Data: frame, Duration: time.Second}); ivfErr != nil {
				panic(ivfErr)
			}
		}
	}()

	// _, iceConnectedCtxCancel := context.WithCancel(context.Background())
	// Set the handler for ICE connection state
	// This will notify you when the peer has connected/disconnected
	// peerConnection.OnICEConnectionStateChange(func(connectionState webrtc.ICEConnectionState) {
	// 	fmt.Printf("Connection State has changed %s \n", connectionState.String())
	// 	if connectionState == webrtc.ICEConnectionStateConnected {
	// 		iceConnectedCtxCancel()
	// 	}
	// })

	// Set the handler for Peer connection state
	// This will notify you when the peer has connected/disconnected
	s.peerConnection.OnConnectionStateChange(func(state webrtc.PeerConnectionState) {
		fmt.Printf("Peer Connection State has changed: %s\n", state.String())

		if state == webrtc.PeerConnectionStateFailed {
			fmt.Println("Peer Connection has gone to failed exiting")
			os.Exit(0)
		}

		if state == webrtc.PeerConnectionStateClosed {
			fmt.Println("Peer Connection has gone to closed exiting")
			os.Exit(0)
		}
	})

	select {}
}

func (s *Streamer) PauseStream() error {

	return nil
}

func (s *Streamer) AddClient(sdp string) (string, error) {
	recvOnlyOffer := webrtc.SessionDescription{}
	decode(sdp, &recvOnlyOffer)

	newClient := SDPClient{SDP: sdp}
	fmt.Println("1")

	// Create a new PeerConnection
	var err error
	newClient.peerConnection, err = webrtc.NewPeerConnection(s.peerConnectionConfig)
	if err != nil {
		return "", err
	}
	fmt.Println("2")

	rtpSender, err := newClient.peerConnection.AddTrack(s.track) //FIXME: this depends on the stream to be started
	if err != nil {
		return "", err
	}
	fmt.Println("3")

	readRTCP(rtpSender)

	// Set the remote SessionDescription
	err = newClient.peerConnection.SetRemoteDescription(recvOnlyOffer) //1
	if err != nil {
		return "", err
	}
	fmt.Println("2")

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
	fmt.Println("3")

	s.clients = append(s.clients, newClient)

	// Block until ICE Gathering is complete, disabling trickle ICE
	// we do this because we only can exchange one signaling message
	// in a production application you should exchange ICE Candidates via OnICECandidate
	fmt.Println("ice wait")
	<-gatherComplete //5
	//<-iceConnectedCtx.Done()
	fmt.Println("ice done")

	// Get the LocalDescription and take it to base64 so we can paste in browser
	newClient.LocalDescription = encode(newClient.peerConnection.LocalDescription())

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
