package main

import (
	"flag"
	"fmt"
	"maps"
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
	pixiPath := flag.String("pixi", "", "Output path to save the Pixi dataset")
	flag.Parse()

	if *pixiPath == "" {
		fmt.Println("Must specify a path to the Pixi GEBCO file")
		return
	}

	file, err := os.Open(*pixiPath)
	if err != nil {
		fmt.Println("Could not open pixi file:", err)
		return
	}
	defer file.Close()

	// read pixi data
	pixiImg, err := pixi.ReadPixi(file)
	if err != nil {
		fmt.Println("Failed to open pixi cache reader", err)
		return
	}

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

	slices.SortFunc(tiles, func(t1, t2 pixigebco.GebcoTile) int {
		if t1.NorthStart.Degree > t2.NorthStart.Degree {
			return -1
		}
		if t1.NorthStart.Degree < t2.NorthStart.Degree {
			return 1
		}
		if t1.WestStart.Degree < t2.WestStart.Degree {
			return -1
		}
		if t1.WestStart.Degree > t2.WestStart.Degree {
			return 1
		}
		return 0
	})

	for _, geotiff := range tiles {
		gFile, err := os.Open(filepath.Join(*tiffsPath, geotiff.FileName))
		if err != nil {
			fmt.Println("failed to open geotiff file:", err)
			return
		}

		geoTileX := (geotiff.WestStart.Degree + 180) / 90
		geoTileY := (-geotiff.NorthStart.Degree / 90) + 1
		fmt.Printf("loading tile: %s, %s at %d, %d...\n", geotiff.NorthStart, geotiff.WestStart, geoTileX, geoTileY)
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

		fmt.Printf("loaded tile: %s, %s at %d, %d with size: %d\n", geotiff.NorthStart, geotiff.WestStart, geoTileX, geoTileY, len(data))
		activeTiles := map[int][]byte{}
		for y := 0; y < int(tags.ImageLength); y++ {
			py := geoTileY*pixigebco.GtiffTileHeight + y
			tileY := py / pixiImg.Layers[0].Dimensions[1].TileSize
			inTileY := py % pixiImg.Layers[0].Dimensions[1].TileSize
			for x := 0; x < int(tags.ImageWidth); x++ {
				px := geoTileX*pixigebco.GtiffTileWidth + x
				tileX := px / pixiImg.Layers[0].Dimensions[0].TileSize
				tileInd := tileY*pixiImg.Layers[0].Dimensions[0].Tiles() + tileX
				if _, ok := activeTiles[tileInd]; !ok {
					if len(activeTiles) >= 40 {
						minInd := slices.Min(slices.Collect(maps.Keys(activeTiles)))
						delete(activeTiles, minInd)
					}
					tileData := make([]byte, pixiImg.Layers[0].DiskTileSize(tileInd))
					err := pixiImg.Layers[0].ReadTile(file, pixiImg.Header, tileInd, tileData)
					if err != nil {
						fmt.Println("failed to read tile", tileInd, err)
						return
					}
					activeTiles[tileInd] = tileData
				}
				tileData := activeTiles[tileInd]
				inTileX := px % pixiImg.Layers[0].Dimensions[0].TileSize
				inTileInd := inTileY*pixiImg.Layers[0].Dimensions[0].TileSize + inTileX
				inTileOffset := inTileInd * pixiImg.Layers[0].SampleSize()
				pVal := pixiImg.Layers[0].Fields[0].BytesToValue(tileData[inTileOffset:], pixiImg.Header.ByteOrder)
				if pVal != int16(data[y*int(tags.ImageWidth)+x]) {
					fmt.Printf("expected value %d at (%d,%d), but got %d at (%d, %d)\n", int16(data[y*int(tags.ImageWidth)+x]), x, y, pVal, px, py)
					return
				}
				if x%2160 == 0 && y%2160 == 0 {
					fmt.Println("x", x, "y", y)
				}
			}
		}
	}

	fmt.Println("all verified!")
}
