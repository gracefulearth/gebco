package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"

	"github.com/owlpinetech/pixi"
	"github.com/owlpinetech/pixi/edit"
	"github.com/owlpinetech/pixi/read"
	pixigebco "github.com/owlpinetech/pixi_gebco"
)

type pixiGebcoTile struct {
	file  *os.File
	pixi  pixi.Pixi
	cache *read.LayerReadCache
}

func main() {
	imgDir := flag.String("dir", "./", "Path to GEBCO GeoTIFF input files, and where the resulting Pixi files will be saved")
	flag.Parse()

	// get all gebco pixi files
	files, err := os.ReadDir(*imgDir) //read the files from the directory
	if err != nil {
		fmt.Println("error reading directory:", err)
		return
	}
	gebcoFiles := []os.DirEntry{}
	for i := range files {
		if strings.HasSuffix(files[i].Name(), ".pixi") {
			gebcoFiles = append(gebcoFiles, files[i])
		}
	}

	// make sure there's the right number
	if len(gebcoFiles) != 8 {
		fmt.Println("missing some input gebco pixi files", len(gebcoFiles))
		return
	}

	pixiGebcoTiles := make([]pixiGebcoTile, len(gebcoFiles))
	for i := range gebcoFiles {
		tileFile, err := os.Open(filepath.Join(*imgDir, gebcoFiles[i].Name()))
		if err != nil {
			fmt.Println("couldn't open tile file", gebcoFiles[i].Name(), err)
			return
		}
		defer tileFile.Close()

		tilePixi, err := pixi.ReadPixi(tileFile)
		if err != nil {
			fmt.Println("failed to read pixi layer", tileFile.Name(), err)
			return
		}

		cache := read.NewLayerReadCache(tileFile, tilePixi.Header, tilePixi.Layers[0], read.NewLfuCacheManager(10))
		pixiGebcoTiles[i] = pixiGebcoTile{file: tileFile, pixi: tilePixi, cache: cache}
	}

	// sort them in row-major order for access later
	slices.SortFunc(pixiGebcoTiles, func(pixOne, pixTwo pixiGebcoTile) int {
		oneNorth, _ := strconv.Atoi(pixOne.pixi.Tags[0].Tags["north"])
		oneWest, _ := strconv.Atoi(pixOne.pixi.Tags[0].Tags["west"])
		twoNorth, _ := strconv.Atoi(pixTwo.pixi.Tags[0].Tags["north"])
		twoWest, _ := strconv.Atoi(pixTwo.pixi.Tags[0].Tags["west"])
		// whichever is more northerly goes first
		if oneNorth > twoNorth {
			return -1
		}
		if oneNorth < twoNorth {
			return 1
		}
		// they're in the same row, so whichever is more westerly goes first
		if oneWest < twoWest {
			return -1
		}
		if oneWest > twoWest {
			return 1
		}
		return 0
	})

	// create a big pixi to stitch the others together
	header := pixi.PixiHeader{Version: pixi.Version, OffsetSize: 8, ByteOrder: binary.BigEndian}
	tags := map[string]string{
		"year":  pixiGebcoTiles[0].pixi.Tags[0].Tags["year"],
		"units": "meters",
	}

	fullFile, err := os.Create(filepath.Join(*imgDir, "gebco_"+pixiGebcoTiles[0].pixi.Tags[0].Tags["year"]+"_sub_ice.pixi"))
	if err != nil {
		fmt.Println("failed to create output file", err)
		return
	}
	err = edit.WriteContiguousTileOrderPixi(fullFile, header, tags, edit.LayerWriter{
		Layer: pixi.NewLayer(
			"gebco_full",
			false,
			pixi.CompressionFlate,
			pixi.DimensionSet{
				{Name: "longitude", Size: pixigebco.GtiffTileWidth * 4, TileSize: pixigebco.GtiffTileWidth / 10},
				{Name: "latitude", Size: pixigebco.GtiffTileHeight * 2, TileSize: pixigebco.GtiffTileHeight / 10},
			},
			[]pixi.Field{{Name: "elevation", Type: pixi.FieldInt16}}),
		IterFn: func(layer *pixi.Layer, coord pixi.SampleCoordinate) ([]any, map[string]any) {
			x := coord[0]
			y := coord[1]
			tileX := x / pixigebco.GtiffTileWidth
			tileY := y / pixigebco.GtiffTileHeight
			tileInd := tileX + (tileY * 4)
			pixiTile := pixiGebcoTiles[tileInd]
			inTileX := x % pixigebco.GtiffTileWidth
			inTileY := y % pixigebco.GtiffTileHeight
			if x%2160 == 0 && y%2160 == 0 {
				fmt.Println("x", x, "y", y, "tileX", tileX, "tileY", tileY, "tileInd", tileInd, "inx", inTileX, "iny", inTileY)
			}

			elev, err := pixiTile.cache.FieldAt(pixi.SampleCoordinate{inTileX, inTileY}, 0)
			if err != nil {
				fmt.Println("failed to read sample", tileInd, x, y, tileX, tileY, err)
				os.Exit(-1)
			}
			return []any{elev.(int16)}, nil
		},
	})

	if err != nil {
		fmt.Println("failed to stich pixi together", err)
	} else {
		fmt.Println("pixi stich done")
	}
}
