package sfu

import (
	"fmt"
	"livekit-video-server/internal/dto"
	"log"

	"github.com/livekit/protocol/livekit"
	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

type SFU struct {
	token string
	url   string
	vch   chan dto.VideoFrame
	room  *lksdk.Room
	track map[int][]*lksdk.LocalTrack
}

func NewManager(url string, token string, vch chan dto.VideoFrame) *SFU {
	return &SFU{
		token: token,
		url:   url,
		vch:   vch,
		room:  nil,
		track: make(map[int][]*lksdk.LocalTrack),
	}
}

func (sfu *SFU) Initialize(nrOfTracks int) error {
	callback := lksdk.NewRoomCallback()
	callback.OnDisconnected = func() {
		// handle disconnect
	}
	callback.OnParticipantConnected = func(participant *lksdk.RemoteParticipant) {
		log.Printf("participant joined: %s", participant.Identity())
	}

	room, err := lksdk.ConnectToRoomWithToken(sfu.url, sfu.token, callback)
	if err != nil {
		return fmt.Errorf("error connecting to room: %w", err)
	}

	sfu.room = room
	mcastID := fmt.Sprintf("mcast-%d", 0)
	for trackIdx := range nrOfTracks + 1 {
		simulcastTracks := make([]*lksdk.LocalTrack, 0, 3)
		for idx := range 3 {
			codec := webrtc.RTPCodecCapability{ //nolint:exhaustruct
				MimeType: webrtc.MimeTypeVP8}
			track, err := lksdk.NewLocalSampleTrack(codec,
				lksdk.WithSimulcast(mcastID, &livekit.VideoLayer{
					Quality: livekit.VideoQuality(idx),
					Width:   1280,
					Height:  720,
				}))
			if err != nil {
				return fmt.Errorf("error creating track: %w", err)
			}
			simulcastTracks = append(simulcastTracks, track)
		}
		sfu.track[trackIdx] = simulcastTracks

		//name := fmt.Sprintf("cam-%d", trackIdx)

		_, err := sfu.room.LocalParticipant.PublishSimulcastTrack(simulcastTracks, &lksdk.TrackPublicationOptions{ //nolint:exhaustruct
			Name:   mcastID,
			Source: 0,
			//VideoWidth:  1920,
			//VideoHeight: 1080,
		})
		if err != nil {
			return fmt.Errorf("error in publish: %w", err)
		}
		// ToDo: Use simulcast for track publishing
		//sfu.room.LocalParticipant.PublishSimulcastTrack()
		//_ = pubtrack.GetSimulcastTrack(livekit.VideoQuality_HIGH)
	}

	return nil
}

func (sfu *SFU) Run() {
	for frame := range sfu.vch {

		if err := sfu.track[frame.Source][frame.Level].WriteSample(media.Sample{ //nolint:exhaustruct
			Data:     frame.Frame,
			Duration: frame.Duration,
		}, nil); err != nil {
			log.Printf("error sending frame: %v", err)
		}
	}
}
