package main

import (
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"path"
	"sync"

	"github.com/gracefulearth/go-colorext"
	"github.com/gracefulearth/gopixi"
	"github.com/gracefulearth/image/tiff"
	"github.com/owlpinetech/gebco"
)

type gebcoLayerTile struct {
	ice    gebco.GebcoTifFile
	subIce gebco.GebcoTifFile
	tid    gebco.GebcoTifFile
}

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
	plusIceFiles := gebco.GebcoTiles(*yearArg, gebco.GebcoDataIce)
	subIceFiles := gebco.GebcoTiles(*yearArg, gebco.GebcoDataSubIce)
	tidFiles := gebco.GebcoTiles(*yearArg, gebco.GebcoDataTypeId)
	allGebcoFiles := make([]gebcoLayerTile, len(plusIceFiles))
	for i := range plusIceFiles {
		// relying on consistent ordering of GebcoTiles function here
		allGebcoFiles[i] = gebcoLayerTile{
			ice:    plusIceFiles[i],
			subIce: subIceFiles[i],
			tid:    tidFiles[i],
		}
	}

	files, err := os.ReadDir(*gebcoSrcArg)
	if err != nil {
		fmt.Printf("failed to read source directory: %v\n", err)
		return
	}

	for _, gebcoFile := range allGebcoFiles {
		foundIce := false
		foundSubIce := false
		foundTid := false
		for _, file := range files {
			if file.Name() == gebcoFile.ice.FileName() {
				foundIce = true
			}
			if file.Name() == gebcoFile.subIce.FileName() {
				foundSubIce = true
			}
			if file.Name() == gebcoFile.tid.FileName() {
				foundTid = true
			}
		}
		if !foundIce {
			fmt.Printf("missing GEBCO file: %s\n", gebcoFile.ice.FileName())
			return
		}
		if !foundSubIce {
			fmt.Printf("missing GEBCO file: %s\n", gebcoFile.subIce.FileName())
			return
		}
		if !foundTid {
			fmt.Printf("missing GEBCO file: %s\n", gebcoFile.tid.FileName())
			return
		}
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

	// iterate over GEBCO tiles and compare against Pixi data
	for gebcoTileIndex, gebcoTile := range allGebcoFiles {
		iceTile, subIceTile, tidTile, err := loadGebcoTileLayer(*gebcoSrcArg, gebcoTile)
		if err != nil {
			fmt.Printf("failed to load GEBCO tile layer: %v\n", err)
			return
		}

		for gebcoTilePixelIndex := range gebco.GtiffSize {
			// calculate x,y of pixel within GEBCO tile
			xInGebcoTile := gebcoTilePixelIndex % gebco.GtiffTileSize
			yInGebcoTile := gebcoTilePixelIndex / gebco.GtiffTileSize

			// calculate global x,y of pixel within full GEBCO dataset
			xGlobal := xInGebcoTile + (gebcoTileIndex%gebco.TilesX)*gebco.GtiffTileSize
			yGlobal := yInGebcoTile + (gebcoTileIndex/gebco.TilesX)*gebco.GtiffTileSize

			// get the pixi sample at this coord
			sample, err := gopixi.SampleAt(readCache, []int{xGlobal, yGlobal})
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
				return
			}
			if sample[1] != gebcoSubIce {
				fmt.Printf("mismatch at (%d,%d) for sub-ice: Pixi=%d GEBCO=%d\n", xGlobal, yGlobal, sample[1], gebcoSubIce)
				return
			}
			if sample[2] != gebcoTid {
				fmt.Printf("mismatch at (%d,%d) for type ID: Pixi=%d GEBCO=%d\n", xGlobal, yGlobal, sample[2], gebcoTid)
				return
			}
		}
	}
}

func loadGebcoTileLayer(folder string, layer gebcoLayerTile) (ice, subIce, tid image.Image, err error) {
	var iceErr, subIceErr, tidErr error

	wg := sync.WaitGroup{}
	wg.Go(func() {
		ice, iceErr = loadGebcoTile(path.Join(folder, layer.ice.FileName()))
	})
	wg.Go(func() {
		subIce, subIceErr = loadGebcoTile(path.Join(folder, layer.subIce.FileName()))
	})
	wg.Go(func() {
		tid, tidErr = loadGebcoTile(path.Join(folder, layer.tid.FileName()))
	})
	wg.Wait()

	if iceErr != nil {
		return nil, nil, nil, iceErr
	}
	if subIceErr != nil {
		return nil, nil, nil, subIceErr
	}
	if tidErr != nil {
		return nil, nil, nil, tidErr
	}
	return ice, subIce, tid, nil
}

func loadGebcoTile(path string) (image.Image, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("failed to open GEBCO file %s: %w", path, err)
	}
	defer file.Close()

	img, err := tiff.Decode(file)
	if err != nil {
		return nil, fmt.Errorf("failed to decode GEBCO file %s: %w", path, err)
	}

	return img, nil
}
