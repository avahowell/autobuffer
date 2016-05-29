package main

import (
	"net/http"
	"os"
	"io"
	"time"
	"strconv"
)
const (
	chunkSize = 1000 // 1MB chunk size
)
// VideoStreamer implements a io.Reader interface.
// It concurrently reads from an underlying HTTP stream, measures available bandwidth,
// and tells the user when they can safely start plaing a video.
type VideoStreamer struct {
	Size int
	tLastChunk time.Time
	rd io.Reader
	out io.Writer
}

// Construct a new video stream from an http URL.
func NewVideoStream(url string, outfile string) (*VideoStreamer, error) {
	res, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	f, err := os.Create(outfile)
	if err != nil {
		return nil, err
	}

	vs := VideoStreamer{}
	sz, err := strconv.Atoi(res.Header["Content-Length"][0])
	if err != nil {
		return nil, err
	}
	vs.Size = sz
	vs.rd = res.Body
	vs.out = f
	return &vs, nil
}

func main() {

}