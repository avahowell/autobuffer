package main

import (
	"flag"
	"fmt"
	"io"
	"io/ioutil"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"
)

// fudgeFactor is used to overestimate buffering time in order to account for
// small variation in available bandwidth over the duration of the stream.
const fudgeFactor = 1.2

// VideoStream streams a remote video to a file over HTTP and informs the user
// when they can start playing the video safely, without interruptions.
type VideoStream struct {
	size     uint64
	duration time.Duration

	f   *os.File
	res *http.Response

	tee io.Reader
}

// NewVideoStream constructs a new video stream from an http URL, duration,
// output path, and optionally HTTP Basic Auth parameters.
func NewVideoStream(url string, duration time.Duration, outfile string, username string, password string) (*VideoStream, error) {
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

	tee := io.TeeReader(res.Body, f)

	return &VideoStream{
		size:     uint64(sz),
		duration: duration,
		tee:      tee,
		res:      res,
		f:        f,
	}, nil
}

// Close closes the underlying file and http response opened by the
// VideoStream.
func (vs *VideoStream) Close() error {
	err := vs.f.Close()
	err = vs.res.Body.Close()
	if err != nil {
		return err
	}
	return nil
}

// bandwidth returns the average bandwidth (in bytes per second) between the
// user and the requested resource.  this bandwidth is computed by downloading up to 10MB.
func (vs *VideoStream) bandwidth() (float64, error) {
	tbefore := time.Now()
	buf := make([]byte, 10000000)
	n, err := io.ReadFull(vs.tee, buf)
	if err != nil && err != io.EOF && err != io.ErrUnexpectedEOF {
		return 0, err
	}
	return float64(n) / (time.Since(tbefore).Seconds()), nil
}

// Stream buffers the remote file into the local file, giving user
// feedback on progress until they can safely play the file.
func (vs *VideoStream) Stream() error {
	fmt.Println("Sampling bandwidth, please wait...")
	bw, err := vs.bandwidth()
	if err != nil {
		return err
	}
	fmt.Printf("Average bandwidth: %v bps\n", bw)

	// Calculate the amount of time needed to safely play the remote video.
	downloadTime := (float64(vs.size) / bw) * fudgeFactor
	bufferTime := time.Duration(math.Max(0, downloadTime-vs.duration.Seconds())) * time.Second

	if bufferTime > 0 {
		fmt.Printf("%v until you can safely watch this video.\n", bufferTime)
		fmt.Println("Buffering...")
	}

	go func() {
		time.Sleep(bufferTime)
		fmt.Printf("%v is now ready to play.\n", vs.f.Name())
	}()

	if _, err := ioutil.ReadAll(vs.tee); err != nil {
		return err
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
		return
	}
	defer vs.Close()

	if err = vs.Stream(); err != nil {
		fmt.Printf("Error streaming %v: %v\n", *videourl, err)
		return
	}
}
