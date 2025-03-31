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

	// "github.com/pion/interceptor"
	// "github.com/pion/interceptor/pkg/intervalpli"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
	"github.com/pion/webrtc/v4/pkg/media/ivfreader"
)

const (
	videoFileName1 = "internal/videos/output1.ivf"
	videoFileName2 = "internal/videos/output3.ivf"
)

type Streamer struct {
	clients              []SDPClient
	PeerConnectionConfig webrtc.Configuration
	readers              []ivfReader
	rIndex               int
	outTrack             *webrtc.TrackLocalStaticSample
	outFrames            chan []byte
	ticker               *time.Ticker //TODO: verify that a single ticker can work

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
	SDP              string
	LocalDescription string
	peerConnection   *webrtc.PeerConnection
}

func (s *Streamer) InitStream() error {
	//if at end of tracks wait for new track to be added
	//set up call back or whatever to handle transitioning the track

	// Asynchronously take all packets in the channel and write them out to our
	// track

	var videoTrackErr error
	s.outTrack, videoTrackErr = webrtc.NewTrackLocalStaticSample(
		webrtc.RTPCodecCapability{MimeType: webrtc.MimeTypeVP8}, "video", "pion",
	)
	if videoTrackErr != nil {
		panic(videoTrackErr)
	}

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
		for {
			if len(s.readers) == 0 {
				continue
			}
			curReader := &s.readers[s.rIndex]
			frame, _, ivfErr := curReader.reader.ParseNextFrame() //TODO: research if you can put frames in memeory or something idk if this is the best
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
		curReader := &s.readers[0]
		s.ticker = time.NewTicker(
			time.Millisecond * time.Duration(
				(float32(curReader.header.TimebaseNumerator)/
					float32(curReader.header.TimebaseDenominator))*1000))
	}

	return nil
}

// TODO:
// abstract the files being read
// add files to que
// starts the stream. blocks untill error
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

	s.clients = append(s.clients, SDPClient{SDP: sdp})
	newClient := &s.clients[len(s.clients)-1]

	// Create a new PeerConnection
	var err error
	newClient.peerConnection, err = webrtc.NewPeerConnection(s.PeerConnectionConfig)
	if err != nil {
		return "", err
	}

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
