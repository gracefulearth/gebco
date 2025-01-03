package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/owlpinetech/pixi"
	"github.com/owlpinetech/pixi/edit"
	pixigebco "github.com/owlpinetech/pixi_gebco"
	"github.com/rngoodner/gtiff"
)

func main() {
	imgDir := flag.String("dir", "./", "Path to GEBCO GeoTIFF input files, and where the resulting Pixi files will be saved")
	flag.Parse()

	// get all gebco geotiff files
	files, err := os.ReadDir(*imgDir) //read the files from the directory
	if err != nil {
		fmt.Println("error reading directory:", err)
		return
	}

	var waitGroup sync.WaitGroup
	for _, file := range files {
		if strings.HasSuffix(file.Name(), ".tif") {
			year, north, south, west, east, err := pixigebco.SplitGebcoFileName(file.Name())
			if err != nil {
				fmt.Println("error extracting file metadata", file.Name(), err)
			} else {
				waitGroup.Add(1)
				go func() {
					defer waitGroup.Done()
					fullName := filepath.Join(*imgDir, strings.TrimSuffix(file.Name(), ".tif"))
					convertGebcoGtiffToPixi(fullName, year, north, south, west, east)
				}()
			}
		}
	}

	waitGroup.Wait()
}

func convertGebcoGtiffToPixi(path string, year int, north int, south int, west int, east int) {
	gtiffFile, err := os.Open(path + ".tif")
	if err != nil {
		fmt.Println("failed to open Gtiff file for reading", path, err)
	}
	defer gtiffFile.Close()

	pixiFile, err := os.Create(path + ".pixi")
	if err != nil {
		fmt.Println("failed to open Pixi file for writing", path, err)
	}
	defer pixiFile.Close()

	base := filepath.Base(path)
	fmt.Println("converting file", base)

	header := pixi.PixiHeader{Version: pixi.Version, OffsetSize: 8, ByteOrder: binary.BigEndian}
	tags := map[string]string{
		"year":  fmt.Sprintf("%d", year),
		"north": fmt.Sprintf("%d", north),
		"south": fmt.Sprintf("%d", south),
		"east":  fmt.Sprintf("%d", east),
		"west":  fmt.Sprintf("%d", west),
		"units": "meters",
	}

	gTags, gHeader, err := gtiff.ReadTags(gtiffFile)
	if err != nil {
		fmt.Println("failed to read geotiff tags:", base, err)
		return
	}
	gData, err := gtiff.ReadData16(gtiffFile, gHeader, gTags)
	if err != nil {
		fmt.Println("failed to read geotiff data:", base, err)
		return
	}

	err = edit.WriteContiguousTileOrderPixi(pixiFile, header, tags,
		edit.LayerWriter{
			Layer: pixi.NewLayer(
				"gebco_full",
				false,
				pixi.CompressionFlate,
				pixi.DimensionSet{
					{Name: "longitude", Size: pixigebco.GtiffTileWidth, TileSize: pixigebco.GtiffTileWidth / 10},
					{Name: "latitude", Size: pixigebco.GtiffTileHeight, TileSize: pixigebco.GtiffTileHeight / 10}},
				[]pixi.Field{{Name: "elevation", Type: pixi.FieldUint16}}),
			IterFn: func(layer *pixi.Layer, coord pixi.SampleCoordinate) ([]any, map[string]any) {
				x := coord[0]
				y := coord[1]
				return []any{gData[x*pixigebco.GtiffTileWidth+y]}, nil
			},
		})

	if err != nil {
		fmt.Println("failed to fully convert gtiff to Pixi", base, err)
	} else {
		fmt.Println("done converting file", base)
	}
}
