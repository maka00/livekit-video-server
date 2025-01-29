package sfu

import (
	"fmt"
	"livekit-video-server/internal/dto"
	"log"

	lksdk "github.com/livekit/server-sdk-go/v2"
	"github.com/pion/webrtc/v4"
	"github.com/pion/webrtc/v4/pkg/media"
)

type SFU struct {
	token string
	url   string
	vch   chan dto.VideoFrame
	room  *lksdk.Room
	track *lksdk.LocalTrack
}

func NewManager(url string, token string, vch chan dto.VideoFrame) *SFU {
	return &SFU{
		token: token,
		url:   url,
		vch:   vch,
		room:  nil,
		track: nil,
	}
}

func (sfu *SFU) Initialize() error {
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

	track, err := lksdk.NewLocalTrack(webrtc.RTPCodecCapability{ //nolint:exhaustruct
		MimeType: webrtc.MimeTypeVP8,
	})

	sfu.track = track

	if err != nil {
		return fmt.Errorf("error creating track: %w", err)
	}

	_, err = sfu.room.LocalParticipant.PublishTrack(track, &lksdk.TrackPublicationOptions{ //nolint:exhaustruct
		Name:   "cam-1",
		Source: 0,
		//VideoWidth:  1920,
		//VideoHeight: 1080,
		DisableDTX: false,
		Stereo:     false,
		Encryption: 0,
	})
	if err != nil {
		return fmt.Errorf("error in publish: %w", err)
	}

	return nil
}

func (sfu *SFU) Run() {
	for frame := range sfu.vch {
		if err := sfu.track.WriteSample(media.Sample{ //nolint:exhaustruct
			Data:     frame.Frame,
			Duration: frame.Duration,
		}, nil); err != nil {
			log.Printf("error sending frame: %v", err)
		}
	}
}
