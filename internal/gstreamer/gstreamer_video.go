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

//export onNeedData
func onNeedData(unused_size C.int, pipelineID C.int) { //nolint:revive,stylecheck
	// log.Printf("NeedData(%d): %d", pipelineID, unused_size)
}

type PipelineKind int

const (
	PipelineKindUnknown = iota
	PipelineKindSending
	PipelineKindReceiving
	PipelineKindSimulcast
	PipelineKindAudioSending
	PipelineKindAudioReveiving
)

type VideoPipeline struct {
	ID       int
	Kind     PipelineKind
	Pipeline string
}

type GstVideo struct {
	ch           chan dto.VideoFrame
	sch          chan dto.VideoFrame
	strPipeline  []string
	pipe         []unsafe.Pointer
	infoPipeline []VideoPipeline
}

func IdentifyPipeline(pipeline string) PipelineKind {
	isSimulcast := strings.Contains(pipeline, "sink_h") &&
		strings.Contains(pipeline, "sink_m") &&
		strings.Contains(pipeline, "sink_l")

	if strings.Contains(pipeline, "appsrc") {
		if strings.Contains(pipeline, "alsasink") {
			return PipelineKindAudioReveiving
		}

		return PipelineKindReceiving
	} else if strings.Contains(pipeline, "appsink") {
		if isSimulcast {
			return PipelineKindSimulcast
		}

		if strings.Contains(pipeline, "alsasrc") {
			return PipelineKindAudioSending
		}

		return PipelineKindSending
	}

	return PipelineKindUnknown
}

var pipeline *GstVideo //nolint:gochecknoglobals
var errPipeline = errors.New("error creating pipeline")

func NewGstVideo(pipelineStr []string, ch chan dto.VideoFrame, sch chan dto.VideoFrame) *GstVideo {
	C.gstreamer_init()

	pipeline = &GstVideo{
		ch:           ch,
		sch:          sch,
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

	for idx, pipe := range gvid.pipe {
		if (gvid.infoPipeline[idx].Kind == PipelineKindReceiving) ||
			(gvid.infoPipeline[idx].Kind == PipelineKindAudioReveiving) {
			continue
		}

		C.gstreamer_start_pipeline(pipe)
	}
}

func (gvid *GstVideo) StartPipeline(id int) error {
	C.gstreamer_start_pipeline(gvid.pipe[id])

	go func() {
		for idx := range gvid.sch {
			duration := idx.Duration.Nanoseconds()
			pipeline := gvid.pipe[idx.Source]
			buffer := unsafe.Pointer(&idx.Frame[0])

			C.gstreamer_push_buffer(pipeline, buffer, C.int(len(idx.Frame)), C.int(duration))
		}
	}()

	return nil
}

func (gvid *GstVideo) StopPipeline(id int) error {
	C.gstreamer_stop_pipeline(gvid.pipe[id])

	return nil
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
