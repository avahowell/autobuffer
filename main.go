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
	chunkSize = 10000 // 10KB chunk size

	// fudgeFactor is used to overestimate buffering time in order to account for
	// small variation in available bandwidth over the duration of the stream.
	fudgeFactor = 1.2
)

// FileStream implements an io.ReadWriteSeeker for a remote file that is being
// streamed to a local file.
type FileStream struct {
	net  io.ReadSeeker
	file io.WriteSeeker
}

// Seek seeks both the remote and the local files to the same offset and
// whence.
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

// Write writes to the local file.
func (fs FileStream) Write(p []byte) (int, error) {
	n, err := fs.file.Write(p)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// Read reads from the remote file.
func (fs FileStream) Read(p []byte) (int, error) {
	n, err := fs.net.Read(p)
	if err != nil {
		return 0, err
	}
	return n, nil
}

// VideoStreamer reads from an underlying HTTP stream, measures available
// bandwidth, buffers the remote file to the local file, and tells the user
// when they can safely start plaing a video.
type VideoStreamer struct {
	Size     int64
	Duration time.Duration
	fs       *FileStream
}

// NewVideoStream constructs a new video stream from an http URL, duration,
// output path, and optionally HTTP Basic Auth parameters.
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
	sz, err := strconv.Atoi(res.Header["Content-Length"][0])
	if err != nil {
		return nil, err
	}

	return &VideoStreamer{
		Size:     int64(sz),
		Duration: duration,
		fs: &FileStream{
			net:  httprs.NewHttpReadSeeker(res, http.DefaultClient),
			file: f,
		},
	}, nil
}

// getBandwidth returns the average bandwidth (in bytes per second) between the
// user and the requested resource.  this bandwidth is computed by downloading
// 2000 chunks.
func (vs *VideoStreamer) getBandwidth() (float64, error) {
	tbefore := time.Now()
	sz := chunkSize * 2000
	buf := make([]byte, sz)
	if _, err := io.ReadFull(vs.fs, buf); err != nil {
		return 0, err
	}
	return float64(sz) / (time.Since(tbefore).Seconds()), nil
}

// StartStream buffers the remote file into the local file, giving user
// feedback on progress until they can safely play the file.
func (vs *VideoStreamer) StartStream() error {
	fmt.Println("Buffering, please wait...")
	bw, err := vs.getBandwidth()
	if err != nil {
		return err
	}
	fmt.Printf("Average bandwidth: %v Bytes per second\n", bw)

	// Calculate the amount of time needed to safely play the remote video.
	downloadTime := (float64(vs.Size) / bw) * fudgeFactor
	bufferTime := math.Max(0, downloadTime-vs.Duration.Seconds())

	fmt.Printf("%v seconds until you can safely watch this video.\n", bufferTime)

	// Download and write the last chunk first so video players can read the file correctly.
	chunk := make([]byte, chunkSize)
	if _, err := vs.fs.Seek(vs.Size-chunkSize, 0); err != nil {
		return err
	}
	io.ReadFull(vs.fs, chunk)
	if _, err := vs.fs.Write(chunk); err != nil {
		return err
	}

	// Start streaming from the start of the file. Once `bufferTime` has elapsed,
	// inform the user that they can safely start playing the file.
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
