package main

import (
	"flag"
	"fmt"
	"math"
	"os"

	"github.com/owlpinetech/healpix"
	"github.com/owlpinetech/pixi"
	pixigebco "github.com/owlpinetech/pixi_gebco"
)

const (
	MaxTileSize = 4096
)

func main() {
	gebcoPath := flag.String("gebco", "./out.pixi", "Input path to save the GEBCO Pixi dataset")
	healPath := flag.String("heal", "./heal.pixi", "Output path to GEBCO Pixi dataset in Healpix layout")
	healRes := flag.Int("res", 9, "The resolution parameter for the healpix map to be generated")
	flag.Parse()

	if *gebcoPath == "" {
		fmt.Println("No image provided, please use -gebco flag.")
		return
	}
	if !healpix.IsValidOrder(*healRes) {
		fmt.Printf("Invalid resolution, must be between 0 and %d\n", healpix.MaxOrder())
		return
	}

	file, err := os.Open(*gebcoPath)
	if err != nil {
		fmt.Println("Could not open pixi file:", err)
		return
	}
	defer file.Close()
	outFile, err := os.Create(*healPath)
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

	// read pixi data into image
	readData, err := pixi.ReadAppend(file, summary, 8)
	if err != nil {
		fmt.Println("Failed to open pixi cache reader", err)
		return
	}

	// create a gebco pixi file, but in healpix layout
	heal := healpix.NewHealpixOrder(*healRes)
	fmt.Printf("Creating HEALPix dataset of order %d (total pixels %d)\n", *healRes, heal.Pixels())

	tileSize := int32(heal.FaceSidePixels())
	for tileSize > MaxTileSize {
		tileSize /= 2
	}
	fmt.Printf("Automatically determined tile size: %d\n", tileSize)

	critSummary := pixi.Summary{
		Metadata: map[string]string{
			"order": fmt.Sprintf("%d", *healRes),
		},
		Separated:   false,
		Compression: pixi.CompressionNone,
		Dimensions: []pixi.Dimension{
			{Size: int64(heal.FaceSidePixels()), TileSize: tileSize}, // face x dimension
			{Size: int64(heal.FaceSidePixels()), TileSize: tileSize}, // face y dimension
			{Size: 12, TileSize: 1}},                                 // face index dimension
		Fields: []pixi.Field{{Name: "elevation", Type: pixi.FieldInt16}},
	}
	critDataset, err := pixi.NewCacheDataset(critSummary, outFile, 8)
	if err != nil {
		fmt.Println("failed to initialize critical pixi dataset for writing:", err)
		return
	}

	for f := int64(0); f < critSummary.Dimensions[2].Size; f++ {
		for y := int64(0); y < critSummary.Dimensions[1].Size; y++ {
			for x := int64(0); x < critSummary.Dimensions[0].Size; x++ {
				latLon := healpix.NewFacePixel(int(f), int(x), int(y)).ToSphereCoordinate(heal)
				gx := lonToX(pixigebco.FromDecimal(toDegrees(latLon.Longitude())))
				gy := latToY(pixigebco.FromDecimal(90 - toDegrees(latLon.Latitude())))
				if gx >= pixigebco.TotalWidth {
					gx = pixigebco.TotalWidth - 1
				}
				if gy >= pixigebco.TotalHeight {
					gy = pixigebco.TotalHeight - 1
				}

				data, err := readData.GetSampleField([]uint{uint(gx), uint(gy)}, 0)
				if err != nil {
					fmt.Printf("Failed to read pixi pixel at: x = %d, y = %d   %v\n", gx, gy, err)
					continue
				}

				err = critDataset.SetSampleField([]uint{uint(x), uint(y), uint(f)}, 0, data.(int16))
				if err != nil {
					fmt.Printf("failed to set healpixel at: x = %d, y = %d, f = %d   %v\n", x, y, f, err)
					continue
				}
			}
			fmt.Println("row done", y)
		}
		fmt.Println("face done", f)
	}
	critDataset.Finalize()
}

func toDegrees(radians float64) float64 {
	return radians * (180.0 / math.Pi)
}

// Converts the GebcoArc to a pixel index along the X axis.
func lonToX(arc pixigebco.GebcoArc) int {
	return arc.Degree*pixigebco.PixelsPerDegree + arc.Minute*pixigebco.PixelsPerMinute + arc.Second/pixigebco.ArcSecIncrement
}

func latToY(arc pixigebco.GebcoArc) int {
	return arc.Degree*pixigebco.PixelsPerDegree + arc.Minute*pixigebco.PixelsPerMinute + arc.Second/pixigebco.ArcSecIncrement
}
