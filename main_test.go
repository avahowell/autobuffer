package main

import (
	"crypto/rand"
	"io"
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"reflect"
	"strconv"
	"testing"
	"time"
)

const (
	testSz       = 50000000
	testFilename = "testout.mkv"
)

var (
	testData = make([]byte, testSz)
)

func TestVideoStreamStream(t *testing.T) {
	os.Remove(testFilename)

	_, err := io.ReadFull(rand.Reader, testData)
	if err != nil {
		t.Fatal(err)
	}

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Length", strconv.Itoa(testSz))
		w.Write(testData)
	}))
	defer ts.Close()

	vs, err := NewVideoStream(ts.URL, time.Second, "testout.mkv", "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer vs.Close()

	if err = vs.Stream(); err != nil {
		t.Fatal(err)
	}

	testf, err := os.Open(testFilename)
	if err != nil {
		t.Fatal(err)
	}
	defer testf.Close()

	data, err := ioutil.ReadAll(testf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data, testData) {
		t.Fatal("data in streamed file did not match testData")
	}

	if err := os.Remove(testFilename); err != nil {
		t.Fatal(err)
	}
}

func TestNewVideoStream(t *testing.T) {
	os.Remove(testFilename)

	_, err := io.ReadFull(rand.Reader, testData)
	if err != nil {
		t.Fatal(err)
	}
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Add("Content-Length", strconv.Itoa(testSz))
		w.Write(testData)
	}))
	defer ts.Close()

	vs, err := NewVideoStream(ts.URL, time.Second, "testout.mkv", "", "")
	if err != nil {
		t.Fatal(err)
	}
	defer vs.Close()

	if vs.size != testSz {
		t.Fatalf("VideoStream created with wrong size, got %v wanted %v\n", vs.size, testSz)
	}
	if vs.duration != time.Second {
		t.Fatal("VideoStream did not set duration")
	}
	if _, err := os.Stat(testFilename); os.IsNotExist(err) {
		t.Fatal("VideoStream did not create outfile")
	}

	data, err := ioutil.ReadAll(vs.tee)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data, testData) {
		t.Fatal("data in vs.tee did not match testData")
	}

	testf, err := os.Open(testFilename)
	if err != nil {
		t.Fatal(err)
	}
	data, err = ioutil.ReadAll(testf)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(data, testData) {
		t.Fatal("data in the output file did not match testData")
	}

	if err := os.Remove(testFilename); err != nil {
		t.Fatal(err)
	}
}
