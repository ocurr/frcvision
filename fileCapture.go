package main

import (
	"bitbucket.org/zombiezen/gocv/cv"
	"fmt"
	"path/filepath"
	"os"
)

type FileCapture struct {
	lastImage *cv.IplImage

	paths []string
	currentPath int
	frameBuff []*cv.IplImage
}

func NewFileCapture(pattern string) (*FileCapture) {
	allPaths, err := filepath.Glob(pattern)
	if err != nil {
		fmt.Println("Failed to find files specified by pattern: ",pattern)
		os.Exit(1)
	}
	fCap := &FileCapture{
		paths: allPaths,
		frameBuff: make([]*cv.IplImage,0),
		currentPath: 0,
	}

	fCap.readFiles()
	return fCap
}

func (fc *FileCapture) readFiles() {
	for i:=0; i<len(fc.paths); i++ {
		img, err := cv.LoadImage(fc.paths[i],cv.LOAD_IMAGE_UNCHANGED)
		if err != nil {
			fmt.Println("Failed to load file: ", fc.paths[i])
			os.Exit(1)
		}
		fc.frameBuff = append(fc.frameBuff, img)
	}
}

func (fc *FileCapture) QueryFrame() *cv.IplImage {

	if len(fc.frameBuff) > fc.currentPath {
		if fc.lastImage != nil {
			fc.lastImage.Release()
			fc.lastImage = nil
		}
		fc.lastImage = fc.frameBuff[fc.currentPath]
		fc.currentPath += 1
	}

	return fc.lastImage
}

