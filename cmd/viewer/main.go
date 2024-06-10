package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"

	"github.com/owlpinetech/pixi"
)

func main() {
	xmin := flag.Int64("xmin", 0, "Coordinate in the dataset that will be the left boundary of the image")
	ymin := flag.Int64("ymin", 0, "Coordinate in the dataset that will be the top boundary of the image")
	xmax := flag.Int64("xmax", 0, "Coordinate in the dataset that will be the right boundary of the image")
	ymax := flag.Int64("ymax", 0, "Coordinate in the dataset that will be the bottom boundary of the image")
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

	if len(summary.Dimensions) != 2 {
		fmt.Println("Viewable pixi files should only contain two dimensions")
		return
	}

	if *xmax > summary.Dimensions[0].Size || *ymax > summary.Dimensions[1].Size {
		fmt.Println("Either xmax or ymax was outside the bounds of the image")
		return
	}

	// read pixi data into image
	readData, err := pixi.ReadAppend(file, summary, 4)
	if err != nil {
		fmt.Println("Failed to open pixi cache reader", err)
		return
	}
	fmt.Println("converting...")

	width := *xmax - *xmin
	height := *ymax - *ymin
	img := image.NewGray16(image.Rect(0, 0, int(width), int(height)))
	minData, maxData := int16(math.MaxInt16), int16(math.MinInt16)
	for y := *ymin; y < *ymax; y++ {
		for x := *xmin; x < *xmax; x++ {
			data, err := readData.GetSampleField([]uint{uint(x), uint(y)}, 0)
			if err != nil {
				fmt.Printf("Failed to read pixi pixel at: x = %d, y = %d   %v\n", x, y, err)
			} else {
				val := data.(int16)
				if val < minData {
					minData = val
				} else if val > maxData {
					maxData = val
				}
				img.SetGray16(int(x-*xmin), int(y-*ymin), color.Gray16{uint16(val)})
			}
		}
	}

	fmt.Println("normalizing...")

	// normalize
	rangeVal := float64(maxData - minData)
	for y := 0; y < int(height); y++ {
		for x := 0; x < int(width); x++ {
			val := img.Gray16At(x, y)
			frac := float64(int16(val.Y)-minData) / rangeVal * math.MaxUint16
			img.SetGray16(x, y, color.Gray16{uint16(frac)})
		}
	}

	fmt.Println("encoding...")
	// write image
	if err = png.Encode(outFile, img); err != nil {
		fmt.Println("failed to encode output:", err)
		return
	}
}
