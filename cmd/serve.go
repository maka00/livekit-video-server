package cmd

import (
	"fmt"
	"livekit-video-server/internal/dto"
	"livekit-video-server/internal/gstreamer"
	sfu "livekit-video-server/internal/sfu"
	"livekit-video-server/internal/token"
	"log"
	"net/url"

	"github.com/spf13/cobra"
	"github.com/spf13/viper"
)

// serveCmd represents the serve command.
var serveCmd = &cobra.Command{ //nolint:exhaustruct,gochecknoglobals
	Use:   "serve",
	Short: "starts the video server and connects to livekit",
	Long:  `Starts the video server and connects to livekit.`,
	Run: func(_ *cobra.Command, _ []string) {
		fmt.Println("serve called")

		tokenSrv := viper.GetString("TOKEN_SERVER")
		lkSrv := viper.GetString("LIVEKIT_SERVER")
		roomID := viper.GetString("ROOM_ID")
		clientID := viper.GetString("CLIENT_ID")

		nrOfPipelines := viper.GetInt("PIPELINES")
		log.Printf("nr of pipelines to start: %d", nrOfPipelines)

		var pipelines []string
		for pipelineID := range nrOfPipelines {
			pipelineEnv := fmt.Sprintf("PIPELINE_%d", pipelineID)
			if pipelineEnv == "" {
				log.Fatalf("PIPELINE_%d not set", pipelineID)
			}
			pipelineStr := viper.GetString(pipelineEnv)
			if pipelineStr == "" {
				log.Fatalf("PIPELINE_%d not set", pipelineID)
			}
			log.Printf("pipeline %d: %s", pipelineID, pipelineStr)
			pipelines = append(pipelines, pipelineStr)
		}

		tokenEndpoint := &url.URL{ //nolint:exhaustruct
			Scheme: "http",
			Host:   tokenSrv,
		}
		tokenEndpoint, err := url.Parse(tokenEndpoint.String())
		if err != nil {
			log.Fatalf("error parsing token endpoint: %v", err)
		}
		vals := url.Values{}

		vals.Add("room", roomID)
		vals.Add("identity", clientID)

		tokenEndpoint.RawQuery = vals.Encode()
		tokenEndpoint.Path = "/token"
		token := token.GetToken(*tokenEndpoint)
		log.Printf("token: %s", token)

		vch := make(chan dto.VideoFrame)
		gst := gstreamer.NewGstVideo(pipelines, vch)
		pipelineInfo, err := gst.Initialize()
		if err != nil {
			log.Fatalf("error initializing gstreamer: %v", err)
		}
		gst.Run()

		lkm := sfu.NewManager(lkSrv, token, vch, pipelineInfo)
		if err := lkm.Initialize(); err != nil {
			log.Fatalf("error initializing livekit: %v", err)
		}

		lkm.Run()
		select {}
	},
}

func init() { //nolint:gochecknoinits
	rootCmd.AddCommand(serveCmd)
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	initConfig()
}
