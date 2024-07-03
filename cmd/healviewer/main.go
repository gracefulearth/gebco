package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"strconv"

	"github.com/owlpinetech/healpix"
	"github.com/owlpinetech/pixi"
)

const (
	arcSecondsPerInc         = 15
	incInterval      float64 = (1.0 / 3600.0) * arcSecondsPerInc
	perDegree                = 60 * (60 / arcSecondsPerInc)
)

func main() {
	xmin := flag.Float64("xmin", 0, "Coordinate in the dataset that will be the left boundary of the image")
	ymin := flag.Float64("ymin", 0, "Coordinate in the dataset that will be the top boundary of the image")
	xmax := flag.Float64("xmax", 0, "Coordinate in the dataset that will be the right boundary of the image")
	ymax := flag.Float64("ymax", 0, "Coordinate in the dataset that will be the bottom boundary of the image")
	src := flag.String("src", "", "Path of the image to view a portion of")
	dest := flag.String("dest", "./dest.png", "Path of the image that will be created")

	flag.Parse()

	if *src == "" {
		fmt.Println("No image provided, please use -src flag.")
		return
	}
	if *xmin >= *xmax || *ymin >= *ymax {
		fmt.Println("xmin should be less than xmax, and ymin should be less than ymax")
		return
	}

	file, err := os.Open(*src)
	if err != nil {
		fmt.Println("Could not open pixi file:", err)
		return
	}
	defer file.Close()
	outFile, err := os.Create(*dest)
	if err != nil {
		fmt.Println("failed to open output file", err)
		return
	}
	defer outFile.Close()

	// read pixi meta
	summary, err := pixi.ReadSummary(file)
	if err != nil {
		fmt.Println("Could not read pixi file summary:", err)
		return
	}

	if *xmax > 180 || *ymax > 90 {
		fmt.Println("Either xmax or ymax was outside the bounds of the image")
		return
	}
	if *xmin < -180 || *ymax < -90 {
		fmt.Println("Either xmin or ymin was outside the bounds of the image")
		return
	}

	// read pixi data into image
	readData, err := pixi.ReadAppend(file, summary, 8)
	if err != nil {
		fmt.Println("Failed to open pixi cache reader", err)
		return
	}
	fmt.Println("converting...")

	order, err := strconv.ParseInt(readData.Metadata["order"], 0, 32)
	if err != nil {
		fmt.Println("unable to get a valid order for the healpix dataset", readData.Metadata["order"])
		return
	}
	fmt.Println("order", order)
	heal := healpix.NewHealpixOrder(int(order))

	byteBuf := []byte{0, 0}

	width := int((*xmax - *xmin) * float64(perDegree))
	height := int((*ymax - *ymin) * float64(perDegree))
	fmt.Printf("creating image of %d x %d\n", width, height)
	img := image.NewNRGBA(image.Rect(0, 0, int(width), int(height)))
	minData, maxData := int16(math.MaxInt16), int16(math.MinInt16)
	for y := *ymax; y >= *ymin; y -= incInterval {
		iy := height - int((y-*ymin)*float64(perDegree)) - 1
		for x := *xmin; x < *xmax; x += incInterval {
			ix := int((x - *xmin) * float64(perDegree))
			ll := healpix.NewLatLonCoordinate(toRadians(y), toRadians(180+x))
			fp := ll.ToFacePixel(heal)

			data, err := readData.GetSampleField([]uint{uint(fp.X()), uint(fp.Y()), uint(fp.Face())}, 0)
			if err != nil {
				fmt.Printf("Failed to read pixi pixel at: x = %d, y = %d, f = %d   %v\n", fp.X(), fp.Y(), fp.Face(), err)
				continue
			}

			final := data.(int16)
			if final < minData {
				minData = final
			} else if final > maxData {
				maxData = final
			}

			binary.BigEndian.PutUint16(byteBuf, uint16(final))
			img.SetNRGBA(ix, iy, color.NRGBA{byteBuf[0], byteBuf[1], 0, 255})
			//fmt.Println("set pixel", ix, iy, x, y, fp.X(), fp.Y(), fp.Face())
		}
	}

	fmt.Println("normalizing...")

	// normalize
	rangeVal := float64(maxData - minData)
	fmt.Println("range", rangeVal, maxData, minData)
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			val := img.NRGBAAt(x, y)
			byteBuf[0] = val.R
			byteBuf[1] = val.G
			crit := val.B
			elev := int16(binary.BigEndian.Uint16(byteBuf))
			frac := byte(float64(elev-minData) / rangeVal * math.MaxUint8)
			switch crit {
			case 2:
				img.SetNRGBA(x, y, color.NRGBA{0, 255, 0, 255})
			case 3:
				img.SetNRGBA(x, y, color.NRGBA{0, 0, 255, 255})
			case 4:
				img.SetNRGBA(x, y, color.NRGBA{255, 0, 0, 255})
			case 5:
				img.SetNRGBA(x, y, color.NRGBA{0, 255, 255, 255})
			default:
				img.SetNRGBA(x, y, color.NRGBA{frac, frac, frac, 255})
			}
		}
	}

	fmt.Println("encoding...")
	// write image
	if err = png.Encode(outFile, img); err != nil {
		fmt.Println("failed to encode output:", err)
		return
	}
}

func toRadians(degrees float64) float64 {
	return degrees * (math.Pi / 180.0)
}
