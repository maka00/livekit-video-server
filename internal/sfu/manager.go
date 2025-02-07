package sfu

import (
	"errors"
	"fmt"
	"livekit-video-server/internal/dto"
	"livekit-video-server/internal/gstreamer"
	"log"
	"strings"
	"time"

	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

type SFU struct {
	ID           string
	token        string
	url          string
	vch          chan dto.VideoFrame
	sch          chan dto.VideoFrame
	room         *lksdk.Room
	track        map[int][]*lksdk.LocalTrack
	pipelineInfo []gstreamer.VideoPipeline
	controller   OutputPipeline
}

type OutputPipeline interface {
	StartPipeline(ID int) error
	StopPipeline(ID int) error
}

func NewManager(instanceID string,
	url string,
	token string,
	vch chan dto.VideoFrame,
	sch chan dto.VideoFrame,
	pipelineInfo []gstreamer.VideoPipeline,
	controller OutputPipeline) *SFU {
	return &SFU{
		ID:           instanceID,
		token:        token,
		url:          url,
		vch:          vch,
		sch:          sch,
		room:         nil,
		track:        make(map[int][]*lksdk.LocalTrack),
		pipelineInfo: pipelineInfo,
		controller:   controller,
	}
}

const (
	layers    = 3
	clockrate = 90000
)

var errPipelineKind = errors.New("error creating track for pipeline kind")

func (sfu *SFU) handleParticipant(participant *lksdk.RemoteParticipant) {
	log.Printf("participant joined: %s", participant.Identity())

	for _, item := range participant.TrackPublications() {
		log.Printf("track: %s", item.SID())
	}
}
func findSendingPipeline(pipelineInfo []gstreamer.VideoPipeline, kind gstreamer.PipelineKind) int {
	for idx, pipeline := range pipelineInfo {
		if pipeline.Kind == kind {
			return idx
		}
	}

	return -1
}

func (sfu *SFU) HandleReceivedFrames(track *webrtc.TrackRemote, pipelineID int) {
	if err := sfu.controller.StartPipeline(pipelineID); err != nil {
		log.Fatalf("error starting pipeline %d: %v", pipelineID, err)
	}

	go func() {
		frame := dto.VideoFrame{} //nolint:exhaustruct
		codecName := strings.Split(track.Codec().RTPCodecCapability.MimeType, "/")[1]
		log.Printf("Track has started, of type %d: %s \n", track.PayloadType(), codecName)

		const bufSize = 1400
		buf := make([]byte, bufSize)

		for {
			start := time.Now()

			nBuffers, _, err := track.Read(buf)
			if err != nil {
				log.Printf("error reading rtp: %v", err)
			}

			frame.Duration = time.Since(start)

			if nBuffers == 0 {
				log.Printf("empty payload")
			}

			frame.Frame = buf[:nBuffers]
			frame.Source = pipelineID
			sfu.sch <- frame
		}
	}()
}

func (sfu *SFU) OnTrackSubscribed(track *webrtc.TrackRemote,
	publication *lksdk.RemoteTrackPublication,
	participant *lksdk.RemoteParticipant) {
	log.Printf("publication: %s track subscribed: %s", participant.Identity(), track.ID())

	if publication.Kind() == lksdk.TrackKindVideo {
		pipelineID := findSendingPipeline(sfu.pipelineInfo, gstreamer.PipelineKindReceiving)
		sfu.HandleReceivedFrames(track, pipelineID)
	}

	if publication.Kind() == lksdk.TrackKindAudio {
		pipelineID := findSendingPipeline(sfu.pipelineInfo, gstreamer.PipelineKindAudioReveiving)
		sfu.HandleReceivedFrames(track, pipelineID)
	}
}

func (sfu *SFU) Initialize() error { //nolint:cyclop
	nrOfTracks := len(sfu.pipelineInfo)
	callback := lksdk.NewRoomCallback()
	callback.OnDisconnected = func() {
		// handle disconnect
	}
	callback.OnParticipantConnected = func(participant *lksdk.RemoteParticipant) {
		sfu.handleParticipant(participant)
	}
	callback.ParticipantCallback.OnTrackSubscribed = sfu.OnTrackSubscribed

	room, err := lksdk.ConnectToRoomWithToken(sfu.url, sfu.token, callback)
	if err != nil {
		return fmt.Errorf("error connecting to room: %w", err)
	}

	sfu.room = room

	for trackIdx := range nrOfTracks {
		mcastID := fmt.Sprintf("%s-%d", sfu.ID, trackIdx)
		log.Printf("creating track with ID: %d", trackIdx)

		switch sfu.pipelineInfo[trackIdx].Kind {
		case gstreamer.PipelineKindSimulcast:
			simulcastTracks, err := sfu.CreateSimulcastTracks(mcastID)
			if err != nil {
				return fmt.Errorf("error creating simulcast tracks: %w", err)
			}

			sfu.track[trackIdx] = simulcastTracks
		case gstreamer.PipelineKindSending:
			track, err := sfu.CreateSingleTrack(mcastID)
			if err != nil {
				return fmt.Errorf("error creating single track: %w", err)
			}

			sfu.track[trackIdx] = []*lksdk.LocalTrack{track}
		case gstreamer.PipelineKindAudioSending:
			track, err := sfu.CreateAudioTrack(mcastID)
			if err != nil {
				return fmt.Errorf("error creating single track: %w", err)
			}

			sfu.track[trackIdx] = []*lksdk.LocalTrack{track}
		case gstreamer.PipelineKindReceiving:
			// do nothing, since we are not sending anything
			break
		case gstreamer.PipelineKindAudioReveiving:
			// do nothing, since we are not sending anything
			break
		default:
			return errPipelineKind
		}
	}

	return nil
}

func (sfu *SFU) CreateAudioTrack(mcastID string) (*lksdk.LocalTrack, error) {
	log.Printf("creating audio track")

	codec := webrtc.RTPCodecCapability{ //nolint:exhaustruct
		MimeType:  webrtc.MimeTypeOpus,
		ClockRate: clockrate,
		RTCPFeedback: []webrtc.RTCPFeedback{
			{Type: webrtc.TypeRTCPFBNACK}, //nolint:exhaustruct
			{Type: webrtc.TypeRTCPFBNACK, Parameter: "pli"},
		},
	}

	track, err := lksdk.NewLocalTrack(codec)
	if err != nil {
		return nil, fmt.Errorf("error creating track: %w", err)
	}

	log.Printf("track created: %s", track.ID())

	_, err = sfu.room.LocalParticipant.PublishTrack(track,
		&lksdk.TrackPublicationOptions{ //nolint:exhaustruct
			Name:   mcastID,
			Source: livekit.TrackSource_MICROPHONE,
		})

	if err != nil {
		return nil, fmt.Errorf("error in publish: %w", err)
	}

	return track, nil
}

func (sfu *SFU) CreateSingleTrack(mcastID string) (*lksdk.LocalTrack, error) {
	log.Printf("creating video track")

	codec := webrtc.RTPCodecCapability{ //nolint:exhaustruct
		MimeType:  webrtc.MimeTypeVP8,
		ClockRate: clockrate,
		RTCPFeedback: []webrtc.RTCPFeedback{
			{Type: webrtc.TypeRTCPFBNACK}, //nolint:exhaustruct
			{Type: webrtc.TypeRTCPFBNACK, Parameter: "pli"},
		},
	}

	track, err := lksdk.NewLocalTrack(codec)
	if err != nil {
		return nil, fmt.Errorf("error creating track: %w", err)
	}

	log.Printf("track created: %s", track.ID())

	_, err = sfu.room.LocalParticipant.PublishTrack(track,
		&lksdk.TrackPublicationOptions{ //nolint:exhaustruct
			Name:   mcastID,
			Source: livekit.TrackSource_CAMERA,
			//VideoWidth:  1920,
			//VideoHeight: 1080,
		})

	if err != nil {
		return nil, fmt.Errorf("error in publish: %w", err)
	}

	return track, nil
}

func (sfu *SFU) CreateSimulcastTracks(mcastID string) ([]*lksdk.LocalTrack, error) {
	log.Printf("creating simulcast tracks")

	simulcastTracks := make([]*lksdk.LocalTrack, 0, layers)

	for idx := range int32(layers) {
		codec := webrtc.RTPCodecCapability{ //nolint:exhaustruct
			MimeType:  webrtc.MimeTypeVP8,
			ClockRate: clockrate,
			RTCPFeedback: []webrtc.RTCPFeedback{
				{Type: webrtc.TypeRTCPFBNACK}, //nolint:exhaustruct
				{Type: webrtc.TypeRTCPFBNACK, Parameter: "pli"},
			},
		}

		track, err := lksdk.NewLocalSampleTrack(codec,
			lksdk.WithSimulcast(mcastID, &livekit.VideoLayer{ //nolint:exhaustruct
				Quality: livekit.VideoQuality(idx),
				//Width:   1280,
				//Height:  720,
			}))
		if err != nil {
			return nil, fmt.Errorf("error creating track: %w", err)
		}

		log.Printf("track created: %s", track.ID())
		simulcastTracks = append(simulcastTracks, track)
	}

	_, err := sfu.room.LocalParticipant.PublishSimulcastTrack(simulcastTracks,
		&lksdk.TrackPublicationOptions{ //nolint:exhaustruct
			Name:   mcastID,
			Source: livekit.TrackSource_CAMERA,
			//VideoWidth:  1920,
			//VideoHeight: 1080,
		})
	if err != nil {
		return nil, fmt.Errorf("error in publish: %w", err)
	}

	return simulcastTracks, nil
}

func (sfu *SFU) Run() {
	go func() {
		for frame := range sfu.vch {
			if err := sfu.track[frame.Source][frame.Level].WriteSample(media.Sample{ //nolint:exhaustruct
				Data:     frame.Frame,
				Duration: frame.Duration,
			}, nil); err != nil {
				log.Printf("error sending frame: %v", err)
			}
		}
	}()
}
