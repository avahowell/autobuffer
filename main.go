package main

import (
	"flag"
	"fmt"
	"github.com/jfbus/httprs"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"
)

const (
	chunkSize   = 10000 // 10KB chunk size
	fudgeFactor = 1.2
)

// FileStream implements a ReadWriteSeeker for a remote file that is being streamed to a local file.
type FileStream struct {
	net  io.ReadSeeker
	file io.WriteSeeker
}

func (fs FileStream) Seek(offset int64, whence int) (int64, error) {
	if _, err := fs.net.Seek(offset, whence); err != nil {
		return 0, err
	}
	n, err := fs.file.Seek(offset, whence)
	if err != nil {
		return 0, err
	}
	return n, nil
}
func (fs FileStream) Write(p []byte) (int, error) {
	n, err := fs.file.Write(p)
	if err != nil {
		return 0, err
	}
	return n, nil
}
func (fs FileStream) Read(p []byte) (int, error) {
	n, err := fs.net.Read(p)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// VideoStreamer concurrently reads from an underlying HTTP stream, measures available bandwidth,
// and tells the user when they can safely start plaing a video.
type VideoStreamer struct {
	Size     int64
	Duration time.Duration
	fs       io.ReadWriteSeeker
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
	vs.fs = FileStream{net: httprs.NewHttpReadSeeker(res, http.DefaultClient), file: f}
	return &vs, nil
}

// getBandwidth returns the average bandwidth (in bytes per second) between the user and the requested resource.
// this bandwidth is computed by downloading 2000 chunks.
func (vs *VideoStreamer) getBandwidth() (float64, error) {
	tbefore := time.Now()
	sz := chunkSize * 2000
	buf := make([]byte, sz)
	if _, err := io.ReadFull(vs.fs, buf); err != nil {
		return 0, err
	}
	return float64(sz) / (time.Since(tbefore).Seconds()), nil
}

// Stream the remote file into the local file, giving user feedback on progress until they can safely play the file.
func (vs *VideoStreamer) StartStream() error {
	fmt.Println("Calculating available downstream bandwidth...")
	bw, err := vs.getBandwidth()
	if err != nil {
		return err
	}
	fmt.Printf("Average bandwidth: %v Bytes per second\n", bw)

	// Calculate the amount of time needed to safely play the remote video.
	downloadTime := (bw / float64(vs.Size)) * fudgeFactor
	bufferTime := math.Max(0, downloadTime-vs.Duration.Seconds())

	fmt.Printf("Now buffering your video. %v seconds until you can safely watch this video.\n", bufferTime)
	chunk := make([]byte, chunkSize)
	// Download and write the last chunk first so video players can read the file correctly.
	if _, err := vs.fs.Seek(vs.Size-chunkSize, 0); err != nil {
		return err
	}
	io.ReadFull(vs.fs, chunk)
	if _, err := vs.fs.Write(chunk); err != nil {
		return err
	}

	// Start streaming from the start of the file.
	if _, err := vs.fs.Seek(0, 0); err != nil {
		return err
	}
	tstart := time.Now()
	readynotified := false
	done := false
	for !done {
		if time.Since(tstart).Seconds() > bufferTime && !readynotified {
			fmt.Println("This video is now ready to play.")
			readynotified = true
		}
		if _, err := io.ReadFull(vs.fs, chunk); err == io.ErrUnexpectedEOF {
			done = true
		} else if err != nil {
			return err
		}
		vs.fs.Write(chunk)
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
