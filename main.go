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
	inputPrefix  = imagePath + "inputs/Input_"
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

	lastTarget Polygon
)

func main() {
	flag.StringVar(&axisHost, "axishost", "10.9.73.20", "Axis camera host")
	flag.StringVar(&axisUsername, "axisuser", "", "Axis camera username")
	flag.StringVar(&axisPassword, "axispass", "", "Axis camera password")
	flag.BoolVar(&captureFile, "file", false, "Should we capture from a file")
	flag.BoolVar(&headless, "headless", false, "should we display output in windowed mode")
	flag.Parse()

	if !captureFile {
		gontlet.InitClient("roborio-973-frc.local:5800")
		visionTable = gontlet.GetTable("vision")
	} else {
		visionTable = nil
	}

	lastTarget = Polygon{make([]cv.Point, 0), cv.Rect{10000000, 1, 1, 1}}

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
			fileCapture = NewFileCapture(imagePath + "inputs/*")
		}

		requestNext := true
		requestPrev := false

		for {

			// Get a frame
			if captureFile && requestNext {
				img = fileCapture.QueryFrame()
				requestNext = false
			} else if captureFile && requestPrev {
				img = fileCapture.QueryLastFrame()
				requestPrev = false
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
						stamp := time.Now().Format("03_04_05_01_02")
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
					if captureFile {
						fileCapture.Close()
					}
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
				} else if key == 'd' {
					requestNext = true
				} else if key == 'a' {
					requestPrev = true
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

type Polygon struct {
	Points []cv.Point
	Bounds cv.Rect
}

func processImage(input *cv.IplImage) (*cv.IplImage, []Polygon) {
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

	//40,80
	cv.Threshold(hue, threshHue, 40, 80, cv.THRESH_BINARY)

	//156,255
	cv.Threshold(sat, threshSat, 156, 255, cv.THRESH_BINARY)

	//183,255
	cv.Threshold(val, threshVal, 183, 255, cv.THRESH_BINARY)

	cv.And(threshHue, threshVal, thresh, nil)
	cv.And(thresh, threshSat, thresh, nil)

	/*
		cv.ShowImage(valWindowName, threshVal)
		cv.ShowImage(satWindowName, threshSat)
		cv.ShowImage(hueWindowName, threshHue)
	*/

	rects := make([]Polygon, 0)
	threshClone := thresh.Clone()
	defer threshClone.Release()
	contour, _ := cv.FindContours(threshClone, storage, cv.RETR_LIST, cv.CHAIN_APPROX_SIMPLE, cv.Point{})
	for ; !contour.IsZero(); contour = contour.Next() {
		result := cv.ApproxPoly(contour, storage, cv.POLY_APPROX_DP, cv.ContourPerimeter(contour)*0.02, 0)
		result = cv.ConvexHull(result, cv.CLOCKWISE, 4)

		if cv.ContourArea(result, cv.WHOLE_SEQ, false) < 100 || cv.ContourArea(result, cv.WHOLE_SEQ, false) >= 70000 || !cv.CheckContourConvexity(result) {
			continue
		}

		var r Polygon
		r.Bounds = cv.BoundingRect(result)
		area := r.Bounds.Height * r.Bounds.Width
		if area < 500 || area > 9000 {
			continue
		}
		for i := 0; i < result.Len(); i++ {
			r.Points = append(r.Points, result.PointAt(i))
		}
		rects = append(rects, r)
	}

	return thresh, rects
}

func processRectangles(rects []Polygon) (Polygon, []Polygon) {

	comparable := Polygon{make([]cv.Point, 0), cv.Rect{10000000, 1, 1, 1}}
	closestRatio := comparable
	closestToOld := comparable

	target := Polygon{make([]cv.Point, 0), cv.Rect{-1, -1, -1, -1}}

	goalRatio := 0.6

	for _, r := range rects {
		/*
			fmt.Println("Goal")
			fmt.Println(float64(r.Bounds.Height) / float64(r.Bounds.Width))
			fmt.Println("Closest")
			fmt.Println(float64(closestRatio.Bounds.Height) / float64(closestRatio.Bounds.Width))
		*/
		if math.Abs(goalRatio-(float64(r.Bounds.Height)/float64(r.Bounds.Width))) < math.Abs(goalRatio-(float64(closestRatio.Bounds.Height)/float64(closestRatio.Bounds.Width))) {
			closestRatio = r
		}

		centerX := r.Bounds.X + r.Bounds.Width/2.0
		centerY := r.Bounds.Y + r.Bounds.Height/2.0
		lastCenterX := lastTarget.Bounds.X + lastTarget.Bounds.Width/2.0
		lastCenterY := lastTarget.Bounds.Y + lastTarget.Bounds.Height/2.0
		closestCenterX := closestToOld.Bounds.X + closestToOld.Bounds.Width/2.0
		closestCenterY := closestToOld.Bounds.Y + closestToOld.Bounds.Height/2.0

		if math.Abs(float64(lastCenterX-centerX)) < math.Abs(float64(lastCenterX-closestCenterX)) && math.Abs(float64(lastCenterY-centerY)) < math.Abs(float64(lastCenterY-closestCenterY)) {
			closestToOld = r
		}
	}

	if closestToOld.Bounds != closestRatio.Bounds {
		target = closestRatio
	} else {
		target = closestToOld
	}

	if target.Bounds == comparable.Bounds {
		lastTarget = comparable
	}

	var centerX float64
	//var centerY float64

	kTargetWidth := 20.0 //inches
	kFOV := 399.0

	//kTargetHeight := 12.0 //inches

	imageCenterX := 320.0 / 2.0
	//imageCenterY := 240/2.0

	centerX = float64(target.Bounds.X + (target.Bounds.Width / 2.0))
	//centerY = float64(target.R.Y + (target.R.Height/2.0))
	xOffsetCenter := centerX - imageCenterX //pix
	//yOffsetCenter := centerY - imageCenterY //pix

	distance := (kTargetWidth * kFOV) / float64(target.Bounds.Width)

	//kB := math.Sin((7.1*math.Pi)/180.0)*(169.0/57)
	//yTheta := (math.Asin(kB*(yOffsetCenter/distance))*180)/math.Pi //degrees
	kA := math.Sin((8.08*math.Pi)/180.0) * (190.0 / 55)
	xTheta := (math.Asin(kA*(xOffsetCenter/distance)) * 180) / math.Pi //degrees

	//fmt.Println("XTheta: ",xTheta)
	//fmt.Println("YTheta: ",yTheta)
	//fmt.Println("Dist: ", distance)
	//fmt.Println("XOff: ",xOffsetCenter)
	//fmt.Println("YOff: ", yOffsetCenter)

	if !captureFile {
		if distance != math.Inf(0) {
			visionTable.Update("found", "true")
			visionTable.Update("xtheta", strconv.FormatFloat(xTheta, 'f', -1, 64))
			visionTable.Update("dist", strconv.FormatFloat(distance, 'f', -1, 64))
		} else {
			visionTable.Update("found", "false")
		}
	}

	for i := 0; i < len(rects); i++ {
		if rects[i].Bounds == target.Bounds {
			rects = append(rects[:i], rects[i+1:]...)
			break
		}
	}

	lastTarget = target

	return target, rects
}

func applyRectangles(img *cv.IplImage, target Polygon, rects []Polygon) *cv.IplImage {
	cpy := img.Clone()

	for _, r := range rects {
		points := make([]cv.Point, 0)
		points = append(points, cv.Point{r.Bounds.X, r.Bounds.Y})
		points = append(points, cv.Point{r.Bounds.X + r.Bounds.Width, r.Bounds.Y})
		points = append(points, cv.Point{r.Bounds.X + r.Bounds.Width, r.Bounds.Y + r.Bounds.Height})
		points = append(points, cv.Point{r.Bounds.X, r.Bounds.Y + r.Bounds.Height})
		cv.PolyLine(cpy, [][]cv.Point{points}, true, cv.Scalar{255.0, 0.0, 0.0, 0.0}, 3, cv.AA, 0)
	}

	//points := target.Points[:]
	points := make([]cv.Point, 0)
	points = append(points, cv.Point{target.Bounds.X, target.Bounds.Y})
	points = append(points, cv.Point{target.Bounds.X + target.Bounds.Width, target.Bounds.Y})
	points = append(points, cv.Point{target.Bounds.X + target.Bounds.Width, target.Bounds.Y + target.Bounds.Height})
	points = append(points, cv.Point{target.Bounds.X, target.Bounds.Y + target.Bounds.Height})
	cv.PolyLine(cpy, [][]cv.Point{points}, true, cv.Scalar{0.0, 0.0, 255.0, 0.0}, 3, cv.AA, 0)

	return cpy
}
