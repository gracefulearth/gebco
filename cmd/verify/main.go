package main

import (
	"flag"
	"fmt"
	"image/color"
	"os"
	"time"

	"github.com/gracefulearth/gebco"
	"github.com/gracefulearth/go-colorext"
	"github.com/gracefulearth/gopixi"
)

func main() {
	pixiSrcArg := flag.String("pixiSrc", "", "Path to source Pixi file to verify")
	gebcoSrcArg := flag.String("gebcoSrc", "", "Path to source GEBCO Geotiff files")
	yearArg := flag.Int("year", 2025, "the GEBCO year to verify against")
	flag.Parse()

	if *pixiSrcArg == "" || *gebcoSrcArg == "" {
		flag.Usage()
		return
	}

	// get GEBCO files
	allGebcoFiles := gebco.GebcoLayeredTiles(*yearArg)

	missing := gebco.CheckDirectoryComplete(*gebcoSrcArg, allGebcoFiles)
	if len(missing) > 0 {
		fmt.Printf("missing %d GEBCO files:\n", len(missing))
		for _, miss := range missing {
			fmt.Printf(" - %s\n", miss)
		}
		return
	}

	// open Pixi file to compare against
	readFile, err := os.Open(*pixiSrcArg)
	if err != nil {
		fmt.Printf("failed to open Pixi file for reading: %v\n", err)
		return
	}

	summary, err := gopixi.ReadPixi(readFile)
	if err != nil {
		fmt.Printf("failed to read Pixi file header: %v\n", err)
		return
	}
	gebcoLayer := summary.Layers[0]
	if gebcoLayer.Name != "gebco" {
		fmt.Printf("expected first layer to be 'gebco', got '%s'\n", gebcoLayer.Name)
		return
	}

	readCache := gopixi.NewFifoCacheReadLayer(readFile, summary.Header, gebcoLayer, 8)
	sample := make(gopixi.Sample, 3)
	// iterate over GEBCO tiles and compare against Pixi data
	for gebcoTileIndex, gebcoTile := range allGebcoFiles {
		iceTile, subIceTile, tidTile, err := gebcoTile.Load(*gebcoSrcArg)
		if err != nil {
			fmt.Printf("failed to load GEBCO tile layer: %v\n", err)
			return
		}
		fmt.Printf("Verifying GEBCO tile %d/%d...\n", gebcoTileIndex+1, len(allGebcoFiles))

		startTime := time.Now()
		for gebcoTilePixelIndex := range gebco.GtiffSize {
			// calculate x,y of pixel within GEBCO tile
			xInGebcoTile := gebcoTilePixelIndex % gebco.GtiffTileSize
			yInGebcoTile := gebcoTilePixelIndex / gebco.GtiffTileSize

			// calculate global x,y of pixel within full GEBCO dataset
			xGlobal := xInGebcoTile + (gebcoTileIndex%gebco.TilesX)*gebco.GtiffTileSize
			yGlobal := yInGebcoTile + (gebcoTileIndex/gebco.TilesX)*gebco.GtiffTileSize

			// get the pixi sample at this coord
			err := gopixi.SampleInto(readCache, []int{xGlobal, yGlobal}, sample)
			if err != nil {
				fmt.Printf("failed to get Pixi sample at (%d,%d): %v\n", xGlobal, yGlobal, err)
				return
			}

			// get GEBCO pixel values
			gebcoIce := iceTile.At(xInGebcoTile, yInGebcoTile).(colorext.GrayS16).Y
			gebcoSubIce := subIceTile.At(xInGebcoTile, yInGebcoTile).(colorext.GrayS16).Y
			gebcoTid := tidTile.At(xInGebcoTile, yInGebcoTile).(color.Gray).Y

			// compare
			if sample[0] != gebcoIce {
				fmt.Printf("mismatch at (%d,%d) for ice: Pixi=%d GEBCO=%d\n", xGlobal, yGlobal, sample[0], gebcoIce)
			}
			if sample[1] != gebcoSubIce {
				fmt.Printf("mismatch at (%d,%d) for sub-ice: Pixi=%d GEBCO=%d\n", xGlobal, yGlobal, sample[1], gebcoSubIce)
			}
			if sample[2] != gebcoTid {
				fmt.Printf("mismatch at (%d,%d) for type ID: Pixi=%d GEBCO=%d\n", xGlobal, yGlobal, sample[2], gebcoTid)
			}

			if (gebcoTilePixelIndex+1)%(gebco.GtiffSize/8) == 0 {
				fmt.Printf("GEBCO tile %d/%d pixels verified: %d/%d\n", gebcoTileIndex+1, len(allGebcoFiles), gebcoTilePixelIndex+1, gebco.GtiffSize)
			}
		}

		totalTileTime := time.Since(startTime)
		fmt.Printf("Verified GEBCO tile %d/%d in %v\n", gebcoTileIndex+1, len(allGebcoFiles), totalTileTime.Seconds())
	}
}
