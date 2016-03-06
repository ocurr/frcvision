package main

import (
	"bitbucket.org/zombiezen/gocv/cv"
	"errors"
	"flag"
	"fmt"
	"github.com/ocurr/gontlet"
	"math"
	"os"
	"strconv"
	"time"
)

const (
	inputWindowName  = "Input"
	outputWindowName = "Output"

	satWindowName = "Saturation"
	valWindowName = "Value"
	hueWindowName = "Hue"

	imagePath    = "images/"
	inputPrefix  = imagePath + "imputs/Input_"
	outputPrefix = imagePath + "outputs/Out_"
	targetPrefix = imagePath + "targets/Target_"
)

var (
	axisHost     string
	axisUsername string
	axisPassword string
	captureFile  bool
	headless     bool
	visionTable  *gontlet.Table
)

func main() {
	flag.StringVar(&axisHost, "axishost", "10.9.73.20", "Axis camera host")
	flag.StringVar(&axisUsername, "axisuser", "", "Axis camera username")
	flag.StringVar(&axisPassword, "axispass", "", "Axis camera password")
	flag.BoolVar(&captureFile, "file", false, "Should we capture from a file")
	flag.BoolVar(&headless, "headless", false, "should we display output in windowed mode")
	flag.Parse()

	gontlet.InitClient("roborio-9973-frc.local:5800")
	visionTable = gontlet.GetTable("vision")

	go run()
	cv.Main()
}

func run() {
	// Create windows
	if !headless {
		cv.NamedWindow(inputWindowName, cv.WINDOW_AUTOSIZE)
		cv.NamedWindow(outputWindowName, cv.WINDOW_AUTOSIZE)
		/*
			cv.NamedWindow(satWindowName, cv.WINDOW_AUTOSIZE)
			cv.NamedWindow(valWindowName, cv.WINDOW_AUTOSIZE)
			cv.NamedWindow(hueWindowName, cv.WINDOW_AUTOSIZE)
		*/
	}

	var err error
	var capture *AxisCamera
	var fileCapture *FileCapture
	var img *cv.IplImage
	saved := false

	for {
		// Set up camera
		if !captureFile {
			err = errors.New("dummy error")
			for err != nil {
				capture, err = NewAxisCamera(axisHost, axisUsername, axisPassword)
				fmt.Fprintln(os.Stderr, "failed to start capture")
				time.Sleep(4 * time.Second)
			}
		} else {
			fileCapture = NewFileCapture(imagePath + "/inputs/*.jpeg")
		}

		advance := true

		for {

			// Get a frame
			if captureFile && advance {
				img = fileCapture.QueryFrame()
				advance = false
			} else if !captureFile {
				img, err = capture.QueryFrame()
			}
			if err != nil {
				fmt.Fprintln(os.Stderr, "failed to query frame")
				break
			}

			if img != nil {
				// Process image
				out, rects := processImage(img)

				target, rest := processRectangles(rects)

				// Display images
				rimg := applyRectangles(img, target, rest)
				if !headless {
					cv.ShowImage(inputWindowName, rimg)
					cv.ShowImage(outputWindowName, out)
				}

				if !captureFile {
					if found, ok := visionTable.GetAsBool("found"); ok && found && !saved {
						stamp := time.Now().Format("03[04]05_01/02")
						cv.SaveImage(inputPrefix+stamp+".jpeg", img)
						cv.SaveImage(outputPrefix+stamp+".jpeg", out)
						cv.SaveImage(targetPrefix+stamp+".jpeg", rimg)
						saved = true
					} else {
						saved = false
					}
				}

				// Wait for input
				key := cv.WaitKey(10 * time.Millisecond)
				if key == 'q' {
					if !headless {
						os.Exit(0)
					}
				} else if key == ' ' {
					if !headless {
						stamp := time.Now().Format("03[04]05_01/02")
						cv.SaveImage(inputPrefix+stamp+".jpeg", img)
						cv.SaveImage(outputPrefix+stamp+".jpeg", out)
						cv.SaveImage(targetPrefix+stamp+".jpeg", rimg)
					}
				} else if key == 'a' {
					advance = true
				}
				if !headless {
					out.Release()
					rimg.Release()
				}
			}
		}
		if !captureFile {
			capture.Close()
		}
	}
	os.Exit(0)
}

func processImage(input *cv.IplImage) (*cv.IplImage, []cv.PointRect) {
	storage := cv.NewMemStorage(0)
	defer storage.Release()

	hsv := cv.NewImage(input.Size(), 8, 3)
	defer hsv.Release()
	hue := cv.NewImage(input.Size(), 8, 1)
	defer hue.Release()
	sat := cv.NewImage(input.Size(), 8, 1)
	defer sat.Release()
	val := cv.NewImage(input.Size(), 8, 1)
	defer val.Release()
	threshSat := cv.NewImage(input.Size(), 8, 1)
	defer threshSat.Release()
	threshVal := cv.NewImage(input.Size(), 8, 1)
	defer threshVal.Release()
	threshHue := cv.NewImage(input.Size(), 8, 1)
	defer threshHue.Release()
	thresh := cv.NewImage(input.Size(), 8, 1)

	cv.CvtColor(input, hsv, cv.BGR2HSV)
	cv.Split(hsv, nil, sat, nil, nil)
	cv.Split(hsv, nil, nil, val, nil)
	cv.Split(hsv, hue, nil, nil, nil)

	//50,255
	cv.Threshold(hue, threshHue, 50, 255, cv.THRESH_BINARY)

	//156,255
	cv.Threshold(sat, threshSat, 156, 255, cv.THRESH_BINARY)

	//183,255
	cv.Threshold(val, threshVal, 100, 255, cv.THRESH_BINARY)

	cv.And(threshHue, threshVal, thresh, nil)
	cv.And(thresh, threshSat, thresh, nil)

	/*
		cv.ShowImage(valWindowName, threshVal)
		cv.ShowImage(satWindowName, threshSat)
		cv.ShowImage(hueWindowName, threshHue)
	*/

	//cv.Dilate(thresh,thresh,nil,2)
	//cv.Erode(thresh,thresh,nil,2)

	rects := make([]cv.PointRect, 0)
	threshClone := thresh.Clone()
	defer threshClone.Release()
	contour, _ := cv.FindContours(threshClone, storage, cv.RETR_LIST, cv.CHAIN_APPROX_SIMPLE, cv.Point{})
	for ; !contour.IsZero(); contour = contour.Next() {
		result := cv.ApproxPoly(contour, storage, cv.POLY_APPROX_DP, cv.ContourPerimeter(contour)*0.02, 0)
		result = cv.ConvexHull(result, cv.CLOCKWISE, 4)

		// result.Len() != 4
		if cv.ContourArea(result, cv.WHOLE_SEQ, false) < 100 || cv.ContourArea(result, cv.WHOLE_SEQ, false) >= 70000 || !cv.CheckContourConvexity(result) {
			continue
		}

		var r cv.PointRect
		r.Rect = cv.BoundingRect(result)
		for i := 0; i < result.Len(); i++ {
			r.Points = append(r.Points, result.PointAt(i))
		}
		rects = append(rects, r)
	}

	return thresh, rects
}

func processRectangles(rects []cv.PointRect) (cv.PointRect, []cv.PointRect) {

	numHoriz := 0
	numVert := 0

	//widest := 0

	var target cv.PointRect

	NEARLY_HORIZONTAL_SLOPE := math.Tan((20 * math.Pi) / 180)
	NEARLY_VERTICAL_SLOPE := math.Tan(((90 - 20) * math.Pi) / 180)

	for _, r := range rects {
		points := r.Points[:]
		for i := 0; i < 4; i++ {
			dy := points[i].Y - points[(i+1)%4].Y
			dx := points[i].X - points[(i+1)%4].X
			slope := 10000000.0
			if dx != 0 {
				slope = math.Abs(float64(dy) / float64(dx))
			}

			if slope < NEARLY_HORIZONTAL_SLOPE {
				numHoriz++
			} else if slope > NEARLY_VERTICAL_SLOPE {
				numVert++
			}
		}

		/*
		if numHoriz >= 1 && numVert == 2 {
			if r.Rect.Width > widest {
				target = r
				widest = r.Rect.Width
			}
		}
		*/
		target = r

		//fmt.Println("Width: ", r.R.Width)
		//fmt.Println("Height: ", r.R.Height)

	}

	var centerX float64
	//var centerY float64

	//kTargetHeight := 12.0 //inches
	kTargetWidth := 20.0 //inches
	kFOV := 399.0

	imageCenterX := 320.0 / 2.0
	//imageCenterY := 240/2.0

	centerX = float64(target.Rect.X + (target.Rect.Width / 2.0))
	//centerY = float64(target.R.Y + (target.R.Height/2.0))
	xOffsetCenter := centerX - imageCenterX //pix
	//yOffsetCenter := centerY - imageCenterY //pix

	distance := (kTargetWidth * kFOV) / float64(target.Rect.Width)

	//kB := math.Sin((7.1*math.Pi)/180.0)*(169.0/57)
	//yTheta := (math.Asin(kB*(yOffsetCenter/distance))*180)/math.Pi //degrees
	kA := math.Sin((8.08*math.Pi)/180.0) * (190.0 / 55)
	xTheta := (math.Asin(kA*(xOffsetCenter/distance)) * 180) / math.Pi //degrees

	//fmt.Println("XTheta: ",xTheta)
	//fmt.Println("YTheta: ",yTheta)
	//fmt.Println("Dist: ", distance)
	//fmt.Println("XOff: ",xOffsetCenter)
	//fmt.Println("YOff: ", yOffsetCenter)

	if distance != math.Inf(0) {
		visionTable.Update("found", "true")
		visionTable.Update("xtheta", strconv.FormatFloat(xTheta, 'f', -1, 64))
		visionTable.Update("dist", strconv.FormatFloat(distance, 'f', -1, 64))
	} else {
		visionTable.Update("found", "false")
	}

	for i := 0; i < len(rects); i++ {
		if rects[i].Rect == target.Rect {
			rects = append(rects[:i], rects[i+1:]...)
			break
		}
	}

	return target, rects
}

func applyRectangles(img *cv.IplImage, target cv.PointRect, rects []cv.PointRect) *cv.IplImage {
	cpy := img.Clone()

	for _, r := range rects {
		points := r.Points[:]
		cv.PolyLine(cpy, [][]cv.Point{points}, true, cv.Scalar{255.0, 0.0, 0.0, 0.0}, 3, cv.AA, 0)
	}

	points := target.Points[:]
	cv.PolyLine(cpy, [][]cv.Point{points}, true, cv.Scalar{0.0, 0.0, 255.0, 0.0}, 3, cv.AA, 0)

	return cpy
}
