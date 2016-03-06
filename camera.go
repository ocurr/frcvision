package main

import (
	"bitbucket.org/zombiezen/gocv/cv"
	"image/jpeg"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"errors"
)

type AxisCamera struct {
	closer io.Closer
	mr     *multipart.Reader

	lastImage *cv.IplImage

	frames chan *cv.IplImage
	quit   chan struct{}
}

func NewAxisCamera(host string, username, password string) (*AxisCamera, error) {
	const path = "/mjpg/video.mjpg"
	req, err := http.NewRequest("GET", "http://"+host+path, nil)
	if err != nil {
		return nil, err
	}
	if username != "" || password != "" {
		// TODO: may need stronger authentication
		req.SetBasicAuth(username, password)
	}

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}

	// TODO: parse this out from Content-Type
	const boundary = "myboundary"
	r := multipart.NewReader(resp.Body, boundary)

	cam := &AxisCamera{
		closer: resp.Body,
		mr:     r,
		frames: make(chan *cv.IplImage),
		quit:   make(chan struct{}),
	}
	go cam.fetchFrames()
	return cam, nil
}

func (cam *AxisCamera) fetchFrames() {
	defer close(cam.frames)
	for {
		select {
		case <-cam.quit:
			return
		default:
			// don't block
		}

		image, err := cam.frame()
		if err != nil {
			log.Println("axis camera error:", err)
			if err == io.EOF {
				image = nil
			}
			continue
		}

		select {
		case <-cam.quit:
			return
		case cam.frames <- image:
			// frame sent, receiver controls memory
		default:
			// receiver is not ready, skip the frame and move on
			image.Release()
		}
	}
}

func (cam *AxisCamera) frame() (*cv.IplImage, error) {
	part, err := cam.mr.NextPart()
	if err != nil {
		return nil, err
	}
	jp, err := jpeg.Decode(part)
	if err != nil {
		return nil, err
	}
	return cv.ConvertImage(jp), nil
}

func (cam *AxisCamera) QueryFrame() (*cv.IplImage, error) {
	if cam.lastImage != nil {
		cam.lastImage.Release()
		cam.lastImage = nil
	}

	img := <-cam.frames
	if img == nil {
		return nil, errors.New("new camera detected")
	}

	cam.lastImage = img
	return cam.lastImage, nil
}

func (cam *AxisCamera) Close() error {
	// Wait for fetchFrames to finish
	cam.quit <- struct{}{}
	close(cam.quit)

	// Close up connection
	if err := cam.closer.Close(); err != nil {
		return err
	}

	// Release last image
	if cam.lastImage != nil {
		cam.lastImage.Release()
		cam.lastImage = nil
	}

	return nil
}
