package main

import (
	"bitbucket.org/zombiezen/gocv/cv"
	"fmt"
	"os"
	"path/filepath"
)

type FileCapture struct {
	lastImage *cv.IplImage

	paths       []string
	currentPath int
	frameBuff   []*cv.IplImage
}

func NewFileCapture(pattern string) *FileCapture {

	fCap := &FileCapture{
		frameBuff:   make([]*cv.IplImage, 0),
		currentPath: 0,
	}

	fileTypes := make(map[int]string, 0)
	fileTypes[0] = ".jpg"
	fileTypes[1] = ".jpeg"
	fileTypes[2] = ".png"
	for i := 0; i < len(fileTypes); i++ {
		fmt.Println("Trying variation: " + fileTypes[i])
		var err error
		fCap.paths, err = filepath.Glob(pattern + fileTypes[i])
		if err != nil {
			fmt.Println("pattern failed. check leading slashes")
			os.Exit(0)
		}
		fCap.readFiles()
		if len(fCap.frameBuff) != 0 {
			fmt.Println("pattern matched loading files")
			return fCap
		}
		fmt.Println("no files found...")
	}

	os.Exit(0)
	return nil
}

func (fc *FileCapture) readFiles() {
	for i := 0; i < len(fc.paths); i++ {
		img, err := cv.LoadImage(fc.paths[i], cv.LOAD_IMAGE_UNCHANGED)
		if err != nil {
			fmt.Println("Failed to load file: ", fc.paths[i])
			os.Exit(1)
		}
		fc.frameBuff = append(fc.frameBuff, img)
	}
}

func (fc *FileCapture) QueryFrame() *cv.IplImage {

	if fc.currentPath >= len(fc.frameBuff) {
		fc.currentPath = 0
	} else if fc.currentPath < 0 {
		fc.currentPath = len(fc.frameBuff) - 1
	}

	if fc.lastImage != nil {
		fc.lastImage.Release()
		fc.lastImage = nil
	}
	fc.lastImage = fc.frameBuff[fc.currentPath].Clone()
	fc.currentPath += 1

	return fc.lastImage
}

func (fc *FileCapture) QueryLastFrame() *cv.IplImage {
	if fc.currentPath >= len(fc.frameBuff) {
		fc.currentPath = 0
	} else if fc.currentPath < 0 {
		fc.currentPath = len(fc.frameBuff) - 1
	}

	if fc.lastImage != nil {
		fc.lastImage.Release()
		fc.lastImage = nil
	}
	fc.lastImage = fc.frameBuff[fc.currentPath].Clone()
	fc.currentPath -= 1

	return fc.lastImage
}

func (fc *FileCapture) Close() {
	for _, i := range fc.frameBuff {
		i.Release()
		i = nil
	}
}
