package dto

import "time"

type Quality int

const (
	Low Quality = iota
	Medium
	High
)

type VideoFrame struct {
	Frame    []byte
	Duration time.Duration
	Source   int
	Level    Quality
}
