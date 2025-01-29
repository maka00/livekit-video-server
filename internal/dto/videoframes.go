package dto

import "time"

type VideoFrame struct {
	Frame    []byte
	Duration time.Duration
	Source   int
}
