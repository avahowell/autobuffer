package main

import (
	"flag"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"
	"github.com/jfbus/httprs"
)

const (
	chunkSize   = 10000     // 10KB chunk size
	testSize    = 30000000 // 30MB test size
	fudgeFactor = 1.2
)

// VideoStreamer concurrently reads from an underlying HTTP stream, measures available bandwidth,
// and tells the user when they can safely start plaing a video.
type VideoStreamer struct {
	Size     int64
	Duration time.Duration
	rs       io.ReadSeeker
	out      io.WriteSeeker
}

// Construct a new video stream from an http URL.
func NewVideoStream(url string, duration time.Duration, outfile string, username string, password string) (*VideoStreamer, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, err
	}
	req.SetBasicAuth(username, password)

	res, err := http.DefaultClient.Do(req)
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
	vs.Size = int64(sz)
	vs.Duration = duration
	vs.rs = httprs.NewHttpReadSeeker(res, http.DefaultClient)
	vs.out = f
	return &vs, nil
}

// Stream the remote file into the local file, giving user feedback on progress until they can safely stream
func (vs *VideoStreamer) StartStream() error {
	// Compute the average maximum downstream speed over 30 chunks
	fmt.Println("Calculating available downstream bandwidth...")

	tBefore := time.Now()
	testBuf := make([]byte, testSize)
	if _, err := io.ReadFull(vs.rs, testBuf); err != nil {
		return err
	}
	elapsedSeconds := time.Since(tBefore).Seconds()
	availableBandwidth := testSize / elapsedSeconds
	downloadTime := (availableBandwidth / float64(vs.Size)) * fudgeFactor
	bufferTime := math.Max(0, downloadTime-vs.Duration.Seconds())

	fmt.Println("Buffering your video...")
	chunk := make([]byte, chunkSize)

	// Download the last 10KB first for video format integrity
	if _, err := vs.rs.Seek(vs.Size - chunkSize, 0); err != nil {
		return err
	}
	if _, err := vs.out.Seek(vs.Size - chunkSize, 0); err != nil {
		return err
	}
	io.ReadFull(vs.rs, chunk)
	if _, err := vs.out.Write(chunk); err != nil {
		return err
	}

	// Start streaming from the start of the file.
	if _, err := vs.rs.Seek(0, 0); err != nil {
		return err
	}
	if _, err := vs.out.Seek(0, 0); err != nil {
		return err
	}
	tBefore = time.Now()
	readynotified := false
	for {
		if time.Since(tBefore).Seconds() > bufferTime && !readynotified {
			fmt.Println("This video is ready to play.")
			readynotified = true
		}
		if _, err := io.ReadFull(vs.rs, chunk); err == io.ErrUnexpectedEOF {
			if _, err := vs.out.Write(chunk); err != nil {
				return err
			}
			break
		}
		if _, err := vs.out.Write(chunk); err != nil {
			return err
		}
	}
	return nil
}

func main() {
	var videourl = flag.String("url", "", "HTTP url of the video to stream")
	var duration = flag.Duration("duration", time.Second, "Duration of the video to stream")
	var outpath = flag.String("out", "out.mkv", "Filepath to stream output")
	var username = flag.String("username", "", "Username to use for HTTP basic auth")
	var password = flag.String("password", "", "Password to user for HTTP basic auth")

	flag.Parse()

	if *videourl == "" || *duration == time.Second {
		fmt.Println("A video url and duration is required for autobuffer.  Usage:")
		flag.PrintDefaults()
		return
	}

	vs, err := NewVideoStream(*videourl, *duration, *outpath, *username, *password)
	if err != nil {
		fmt.Printf("Error creating video stream: %v\n", err)
	}
	if err = vs.StartStream(); err != nil {
		fmt.Printf("Error streaming %v: %v\n", *videourl, err)
	}
}
