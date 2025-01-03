package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/owlpinetech/pixi"
	pixigebco "github.com/owlpinetech/pixi_gebco"
	"github.com/rngoodner/gtiff"
)

func main() {
	tiffsPath := flag.String("geotiff", "./", "Input path to GEBCO GeoTIFF files")
	pixiPath := flag.String("pixi", "./out.pixi", "Output path to save the Pixi dataset")
	flag.Parse()

	if *tiffsPath == "" {
		fmt.Println("No image provided, please use -src flag.")
		return
	}

	file, err := os.Open(*pixiPath)
	if err != nil {
		fmt.Println("Could not open pixi file:", err)
		return
	}
	defer file.Close()

	// read pixi meta
	pixiGebcoTile, err := pixi.ReadPixi(file)
	if err != nil {
		fmt.Println("Could not read pixi file summary:", err)
		return
	}

	fmt.Println("pixi gebco tile data size", pixiGebcoTile.Layers[0].DataSize())

	// read pixi data into image
	readData, err := pixi.ReadAppend(file, pixiGebcoTile, 4)
	if err != nil {
		fmt.Println("Failed to open pixi cache reader", err)
		return
	}
	fmt.Println("converting...")
	fmt.Println(readData.Summary)

	// get all gebco geotiff files
	files, err := os.ReadDir(*tiffsPath) //read the files from the directory
	if err != nil {
		fmt.Println("error reading directory:", err) //print error if directory is not read properly
		return
	}

	tiles := []pixigebco.GebcoTile{}
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".tif") {
			fname, _ := strings.CutSuffix(file.Name(), ".tif")
			split := strings.Split(fname, "_")
			northDeg, err := strconv.ParseInt(strings.Split(split[4][1:], ".")[0], 0, 0)
			if err != nil {
				fmt.Println("failed to extract north from gebco file name:", err)
				return
			}
			westDeg, err := strconv.ParseInt(strings.Split(split[6][1:], ".")[0], 0, 0)
			if err != nil {
				fmt.Println("failed to extract west from gebco file name:", err)
				return
			}
			tile := pixigebco.GebcoTile{
				FileName:   file.Name(),
				NorthStart: pixigebco.GebcoArc{Degree: int(northDeg)},
				WestStart:  pixigebco.GebcoArc{Degree: int(westDeg)},
			}
			tiles = append(tiles, tile)
			fmt.Println(tile)
		}
	}
	// sort the north to south, west to east
	slices.SortFunc(tiles, func(t1 pixigebco.GebcoTile, t2 pixigebco.GebcoTile) int {
		if t1.NorthStart.Degree == t2.NorthStart.Degree {
			if t1.WestStart.Degree < t2.WestStart.Degree {
				return -1
			}
			return 1
		} else if t1.NorthStart.Degree < t2.NorthStart.Degree {
			return 1
		}
		return -1
	})

	for _, geotiff := range tiles {
		gFile, err := os.Open(filepath.Join(*tiffsPath, geotiff.FileName))
		if err != nil {
			fmt.Println("failed to open geotiff file:", err)
			return
		}

		tags, header, err := gtiff.ReadTags(gFile)
		if err != nil {
			fmt.Println("failed to read geotiff tags:", err)
			return
		}
		data, err := gtiff.ReadData16(gFile, header, tags)
		if err != nil {
			fmt.Println("failed to read geotiff data:", err)
			return
		}
		fmt.Printf("loaded tile: %s, %s with size: %d\n", geotiff.NorthStart, geotiff.WestStart, len(data))

		geoTileX := (geotiff.WestStart.Degree + 180) / 90
		geoTileY := (-geotiff.NorthStart.Degree / 90) + 1
		for x := 0; x < int(tags.ImageWidth); x++ {
			for y := 0; y < int(tags.ImageLength); y++ {
				px := geoTileX*21600 + x
				py := geoTileY*21600 + y
				pVal, err := readData.GetSampleField([]uint{uint(px), uint(py)}, 0)
				if err != nil {
					fmt.Println("failed to read pixi pixel:", px, py, err)
					return
				}
				if pVal != int16(data[y*int(tags.ImageWidth)+x]) {
					fmt.Printf("expected value %d at (%d,%d), but got %d\n", int16(data[y*int(tags.ImageWidth)+x]), px, py, pVal)
					return
				}
			}
		}
	}
}
