package gstreamer

/*
#cgo pkg-config: gstreamer-1.0 gstreamer-app-1.0
#include "gstreamer_wrapper.h"
*/
import "C"
import (
	"errors"
	"livekit-video-server/internal/dto"
	"log"
	"os"
	"strings"
	"time"
	"unsafe"
)

//export onBusMessage
func onBusMessage(msgType *C.char, msg *C.char, id C.int) {
	log.Printf("BusMessage(%d): %s(%s)", id, C.GoString(msgType), C.GoString(msg))
	os.Exit(1)
}

//export onNewFrame
func onNewFrame(frame unsafe.Pointer, size C.int, duration C.int, pipelineID C.int, quality C.int) {
	frameBytes := C.GoBytes(frame, size) //nolint:nlreturn
	qualityLevel := int(quality)

	pipeline.ch <- dto.VideoFrame{
		Frame:    frameBytes,
		Duration: time.Duration(duration),
		Source:   int(pipelineID),
		Level:    dto.Quality(qualityLevel),
	}
}

type PipelineKind int

const (
	PipelineKindUnknown = iota
	PipelineKindSending
	PipelineKindReceiving
	PipelineKindSimulcast
	PipelineKindAudio
)

type VideoPipeline struct {
	ID       int
	Kind     PipelineKind
	Pipeline string
}

type GstVideo struct {
	ch           chan dto.VideoFrame
	strPipeline  []string
	pipe         []unsafe.Pointer
	infoPipeline []VideoPipeline
}

func IdentifyPipeline(pipeline string) PipelineKind {
	if strings.Contains(pipeline, "appsrc") {
		return PipelineKindReceiving
	} else if strings.Contains(pipeline, "appsink") {
		if strings.Contains(pipeline, "sink_h") &&
			strings.Contains(pipeline, "sink_m") &&
			strings.Contains(pipeline, "sink_l") {
			return PipelineKindSimulcast
		} else if strings.Contains(pipeline, "alsasrc") {
			return PipelineKindAudio
		}

		return PipelineKindSending
	}

	return PipelineKindUnknown
}

var pipeline *GstVideo //nolint:gochecknoglobals
var errPipeline = errors.New("error creating pipeline")

func NewGstVideo(pipelineStr []string, ch chan dto.VideoFrame) *GstVideo {
	C.gstreamer_init()

	pipeline = &GstVideo{
		ch:           ch,
		strPipeline:  pipelineStr,
		pipe:         make([]unsafe.Pointer, 0),
		infoPipeline: make([]VideoPipeline, 0),
	}

	return pipeline
}

func (gvid *GstVideo) Initialize() ([]VideoPipeline, error) {
	for idx, pipeline := range gvid.strPipeline {
		pipelineCString := C.CString(pipeline)

		defer C.free(unsafe.Pointer(pipelineCString)) //nolint:nlreturn

		singlePipe := C.gstreamer_prepare_pipelines(pipelineCString, C.int(idx))
		if singlePipe == nil {
			return nil, errPipeline
		}

		gvid.infoPipeline = append(gvid.infoPipeline,
			VideoPipeline{ID: idx, Kind: IdentifyPipeline(pipeline), Pipeline: pipeline})

		gvid.pipe = append(gvid.pipe, singlePipe)
	}

	return gvid.infoPipeline, nil
}

func (gvid *GstVideo) Run() {
	go func() {
		C.gstreamer_start_main_loop()
	}()

	for _, pipe := range gvid.pipe {
		C.gstreamer_start_pipeline(pipe)
	}
}

func (gvid *GstVideo) Stop() {
	for _, pipe := range gvid.pipe {
		C.gstreamer_stop_pipeline(pipe)
	}

	C.gstreamer_stop_main_loop()
}

func (gvid *GstVideo) Dispose() {
	C.gstreamer_deinit()
}
