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
	factor := flag.Int("factor", 10, "Amount the image will be downscaled by (division)")
	src := flag.String("src", "", "Path of the image to view a portion of")
	dest := flag.String("dest", "./dest.png", "Path of the image that will be created")

	flag.Parse()

	if *src == "" {
		fmt.Println("No image provided, please use -src flag.")
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

	// read pixi data into image
	readData, err := pixi.ReadAppend(file, summary, 16)
	if err != nil {
		fmt.Println("Failed to open pixi cache reader", err)
		return
	}
	fmt.Println("downsampling...")

	widthTiles := int(summary.Dimensions[0].Size) / *factor
	heightTiles := int(summary.Dimensions[1].Size) / *factor
	img := image.NewGray16(image.Rect(0, 0, int(widthTiles), int(heightTiles)))
	minData, maxData := int16(math.MaxInt16), int16(math.MinInt16)
	for xTile := 0; xTile < widthTiles; xTile++ {
		for yTile := 0; yTile < heightTiles; yTile++ {
			avg := float64(0)
			for xCoord := 0; xCoord < *factor; xCoord++ {
				for yCoord := 0; yCoord < *factor; yCoord++ {
					x := uint(xCoord + xTile*(*factor))
					y := uint(yCoord + yTile*(*factor))
					data, err := readData.GetSampleField([]uint{x, y}, 0)
					if err != nil {
						fmt.Printf("Failed to read pixi pixel at: x = %d, y = %d   %v\n", x, y, err)
					}
					val := data.(int16)
					avg += float64(val)
				}
			}
			avg /= float64(*factor * *factor)
			final := int16(avg)
			if final < minData {
				minData = final
			} else if final > maxData {
				maxData = final
			}
			img.SetGray16(int(xTile), int(yTile), color.Gray16{uint16(avg)})
		}
		fmt.Println("row done", xTile)
	}

	fmt.Println("normalizing...")

	// normalize
	rangeVal := float64(maxData - minData)
	for y := 0; y < int(heightTiles); y++ {
		for x := 0; x < int(widthTiles); x++ {
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
