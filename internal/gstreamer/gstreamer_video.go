package gstreamer

/*
#cgo pkg-config: gstreamer-1.0 gstreamer-app-1.0
#include "gstreamer_wrapper.h"
*/
import "C"
import (
	"errors"
	"livekit-video-server/internal/dto" //nolint:typecheck
	"log"
	"os"
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

type GstVideo struct {
	ch          chan dto.VideoFrame
	strPipeline []string
	pipe        []unsafe.Pointer
}

var pipeline *GstVideo //nolint:gochecknoglobals
var errPipeline = errors.New("error creating pipeline")

func NewGstVideo(pipelineStr []string, ch chan dto.VideoFrame) *GstVideo {
	C.gstreamer_init()

	pipeline = &GstVideo{
		ch:          ch,
		strPipeline: pipelineStr,
		pipe:        make([]unsafe.Pointer, 0),
	}

	return pipeline
}

func (gvid *GstVideo) Initialize() error {
	for id, pipeline := range gvid.strPipeline {
		pipelineCString := C.CString(pipeline)

		defer C.free(unsafe.Pointer(pipelineCString)) //nolint:nlreturn

		singlePipe := C.gstreamer_prepare_pipelines(pipelineCString, C.int(id))
		if singlePipe == nil {
			return errPipeline
		}

		gvid.pipe = append(gvid.pipe, singlePipe)
	}

	return nil
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
