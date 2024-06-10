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
	tiffsPath := flag.String("src", "./", "Input path to GEBCO GeoTIFF files")
	pixiPath := flag.String("out", "./out.pixi", "Output path to save the Pixi dataset")
	flag.Parse()

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

	// open append pixi file
	gebcoSummary := pixi.Summary{
		Metadata: map[string]string{
			"dimOne": "longitude",
			"dimTwo": "latitude",
		},
		Separated:   false,
		Compression: pixi.CompressionFlate,
		Dimensions:  []pixi.Dimension{{Size: 86400, TileSize: 21600 / 4}, {Size: 43200, TileSize: 21600 / 4}},
		Fields:      []pixi.Field{{Name: "elevation", Type: pixi.FieldInt16}},
	}
	file, err := os.Create(*pixiPath)
	if err != nil {
		fmt.Println("failed to open output pixi file for writing:", err)
		return
	}
	defer file.Close()

	gebcoAppend, err := pixi.NewAppendDataset(gebcoSummary, file, 4)
	if err != nil {
		fmt.Println("failed to initialize gebco pixi dataset for writing:", err)
		return
	}

	xTiles := gebcoAppend.Dimensions[0].Tiles()
	yTiles := gebcoAppend.Dimensions[1].Tiles()
	xTilesPerGebco := xTiles / 4
	yTilesPerGebco := yTiles / 2
	fmt.Println("xTilesPerGebco:", xTilesPerGebco)
	fmt.Println("yTilesPerGebco:", yTilesPerGebco)

	// read geotiff and write to pixi
	currentGeoInd := -1
	currentTiff := pixigebco.GebcoTile{}
	data := []uint16{}
	for yTile := 0; yTile < gebcoAppend.Dimensions[1].Tiles(); yTile++ {
		for xTile := 0; xTile < gebcoAppend.Dimensions[0].Tiles(); xTile++ {
			geotiffInd := xTile/xTilesPerGebco + yTile/yTilesPerGebco*xTilesPerGebco
			if geotiffInd != currentGeoInd {
				currentTiff = tiles[geotiffInd]
				gFile, err := os.Open(filepath.Join(*tiffsPath, currentTiff.FileName))
				if err != nil {
					fmt.Println("failed to open geotiff file:", err)
					return
				}

				tags, header, err := gtiff.ReadTags(gFile)
				if err != nil {
					fmt.Println("failed to read geotiff tags:", err)
					return
				}
				data, err = gtiff.ReadData16(gFile, header, tags)
				if err != nil {
					fmt.Println("failed to read geotiff data:", err)
					return
				}
				fmt.Printf("loaded tile: %s, %s with size: %d\n", currentTiff.NorthStart, currentTiff.WestStart, len(data))
				currentGeoInd = geotiffInd
			}

			geoTileX := (currentTiff.WestStart.Degree + 180) / 90
			geoTileY := (-currentTiff.NorthStart.Degree / 90) + 1
			geoSubTileX := xTile % xTilesPerGebco
			geoSubTileY := yTile % xTilesPerGebco
			fmt.Printf("geoX: %d, geoY: %d, geoSubX: %d, geoSubY: %d, x: %d, y: %d\n", geoTileX, geoTileY, geoSubTileX, geoSubTileY, xTile, yTile)
			for yPix := 0; yPix < int(gebcoAppend.Dimensions[1].TileSize); yPix++ {
				for xPix := 0; xPix < int(gebcoAppend.Dimensions[0].TileSize); xPix++ {
					gx := geoSubTileX*int(gebcoAppend.Dimensions[0].TileSize) + xPix
					gy := geoSubTileY*int(gebcoAppend.Dimensions[1].TileSize) + yPix
					px := uint(xTile*int(gebcoAppend.Dimensions[0].TileSize) + xPix)
					py := uint(yTile*int(gebcoAppend.Dimensions[1].TileSize) + yPix)
					d := data[gx+gy*21600]
					err := gebcoAppend.SetSampleField([]uint{px, py}, 0, int16(d))
					if err != nil {
						fmt.Printf("Unable to set pixel at %d,%d (tiff %d @ %d,%d) - %v\n", px, py, currentGeoInd, gx, gy, err)
						return
					}
				}
			}
		}
	}

	gebcoAppend.Finalize()
}
