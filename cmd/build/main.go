package main

import (
	"encoding/binary"
	"flag"
	"fmt"
	"image"
	"image/color"
	"os"
	"strconv"

	"github.com/gracefulearth/gebco"
	"github.com/gracefulearth/go-colorext"
	"github.com/gracefulearth/gopixi"
)

func main() {
	srcArg := flag.String("src", "", "Path to source GEBCO Geotiff files")
	dstArg := flag.String("dst", "", "Path to output stitched GEBCO Pixi file")
	yearArg := flag.Int("year", 2025, "the GEBCO year to build the Pixi file from")
	tileSizeArg := flag.Int("tileSize", gebco.GtiffTileSize/8, "the size of tiles to generate in the Pixi file (must be a divisor of GEBCO tile size = 21600)")
	compressionArg := flag.Int("compression", 1, "compression to be used for data in Pixi (none, flate, lzw-lsb, lzw-msb, rle8) represented as 0, 1, 2, 3, 4 respectively")
	planarArg := flag.Bool("planar", false, "whether to use planar (separated) or interleaved channel storage in the Pixi file")
	orderArg := flag.String("endian", "native", "the endianness byte order (big, little, native) to use in the Pixi file")
	overviewSizeArg := flag.Int("overviewSize", gebco.GtiffTileSize/10, "the size of the overview layer tiles to generate in the Pixi file")
	flag.Parse()

	// validate arguments
	if *srcArg == "" || *dstArg == "" {
		flag.Usage()
		return
	}

	if *tileSizeArg <= 0 || *tileSizeArg > gebco.GtiffTileSize || (gebco.GtiffTileSize%*tileSizeArg) != 0 || (gebco.GtiffTileSize / *tileSizeArg) == 0 {
		fmt.Printf("invalid tile size argument: %d\n", *tileSizeArg)
		return
	}

	compression := gopixi.CompressionNone
	switch *compressionArg {
	case 0:
		compression = gopixi.CompressionNone
	case 1:
		compression = gopixi.CompressionFlate
	case 2:
		compression = gopixi.CompressionLzwLsb
	case 3:
		compression = gopixi.CompressionLzwMsb
	case 4:
		compression = gopixi.CompressionRle8
	default:
		fmt.Printf("invalid compression argument: %d\n", *compressionArg)
		return
	}

	var order binary.ByteOrder
	switch *orderArg {
	case "big":
		order = binary.BigEndian
	case "little":
		order = binary.LittleEndian
	case "native":
		order = binary.NativeEndian
	default:
		fmt.Printf("invalid endianness argument: %s\n", *orderArg)
		return
	}

	// get GEBCO files
	allGebcoFiles := gebco.GebcoLayeredTiles(*yearArg)

	missing := gebco.CheckDirectoryComplete(*srcArg, allGebcoFiles)
	if len(missing) > 0 {
		fmt.Printf("missing %d GEBCO files:\n", len(missing))
		for _, miss := range missing {
			fmt.Printf(" - %s\n", miss)
		}
		return
	}

	// create destination Pixi file
	pixiFile, err := os.Create(*dstArg)
	if err != nil {
		fmt.Printf("failed to create destination Pixi file: %v\n", err)
		return
	}
	defer pixiFile.Close()

	summary := &gopixi.Pixi{
		Header: gopixi.NewHeader(order, gopixi.OffsetSize8),
	}
	if err := summary.Header.WriteHeader(pixiFile); err != nil {
		fmt.Printf("failed to write Pixi header: %v\n", err)
		return
	}

	err = summary.AppendTags(pixiFile, map[string]string{"year": strconv.Itoa(*yearArg)})
	if err != nil {
		fmt.Printf("failed to write Pixi tags: %v\n", err)
		return
	}

	// add the high resolution layer
	opts := []gopixi.LayerOption{gopixi.WithCompression(compression)}
	if *planarArg {
		opts = append(opts, gopixi.WithPlanar())
	}
	highResLayer := gopixi.NewLayer("gebco",
		gopixi.DimensionSet{
			{Name: "lng", TileSize: *tileSizeArg, Size: gebco.TotalWidth},
			{Name: "lat", TileSize: *tileSizeArg, Size: gebco.TotalHeight}},
		gopixi.ChannelSet{
			{Name: "ice", Type: gopixi.ChannelInt16},
			{Name: "sub-ice", Type: gopixi.ChannelInt16},
			{Name: "tid", Type: gopixi.ChannelUint8}},
		opts...,
	)

	gebcoTileTracker := -1
	var gebcoIceTile image.Image
	var gebcoSubIceTile image.Image
	var gebcoTidTile image.Image

	iterator := gebco.NewGebcoTileOrderWriteIterator(pixiFile, summary.Header, highResLayer)
	err = summary.AppendIterativeLayer(pixiFile, highResLayer, iterator, func(dstIterator gopixi.IterativeLayerWriter) error {
		for dstIterator.Next() {
			coord := dstIterator.Coordinate()
			gebcoTile := coord[0]/gebco.GtiffTileSize + (coord[1]/gebco.GtiffTileSize)*gebco.TilesX
			xGebcoTile := gebcoTile % gebco.TilesX
			yGebcoTile := gebcoTile / gebco.TilesX

			if gebcoTile != gebcoTileTracker {
				gebcoTileTracker = gebcoTile
				gebcoFile := allGebcoFiles[gebcoTileTracker]

				fmt.Println("Loading GEBCO layer tile:", xGebcoTile, yGebcoTile)
				gebcoIceTile, gebcoSubIceTile, gebcoTidTile, err = gebcoFile.Load(*srcArg)
				if err != nil {
					return fmt.Errorf("failed to load GEBCO tile layer: %w", err)
				}
				fmt.Println("Loaded GEBCO layer tile")
			} else {
				gebcoTilePixel := (coord[0] % gebco.GtiffTileSize) + (coord[1]%gebco.GtiffTileSize)*gebco.GtiffTileSize
				if gebcoTilePixel%(gebco.GtiffSize/8) == 0 {
					fmt.Println("GEBCO tile pixels processed:", gebcoTilePixel, "/", gebco.GtiffSize)
				}
			}

			xInGebcoTile := coord[0] - (xGebcoTile * gebco.GtiffTileSize)
			yInGebcoTile := coord[1] - (yGebcoTile * gebco.GtiffTileSize)
			iceValue := gebcoIceTile.At(xInGebcoTile, yInGebcoTile).(colorext.GrayS16).Y
			subIceValue := gebcoSubIceTile.At(xInGebcoTile, yInGebcoTile).(colorext.GrayS16).Y
			tidValue := gebcoTidTile.At(xInGebcoTile, yInGebcoTile).(color.Gray).Y

			dstIterator.SetSample(gopixi.Sample{iceValue, subIceValue, tidValue})
		}
		return nil
	})

	if err != nil {
		fmt.Printf("failed to write Pixi layer: %v\n", err)
		return
	}

	// add the overview layer
	fmt.Println("Generating overview layer...")
	readFile, err := os.Open(*dstArg)
	if err != nil {
		fmt.Printf("failed to open Pixi file for reading: %v\n", err)
		return
	}

	readCache := gopixi.NewFifoCacheReadLayer(readFile, summary.Header, highResLayer, 8)
	overviewLayer := gopixi.NewLayer("gebco_overview",
		gopixi.DimensionSet{
			{Name: "lng", TileSize: *overviewSizeArg, Size: *overviewSizeArg * gebco.TilesX},
			{Name: "lat", TileSize: *overviewSizeArg, Size: *overviewSizeArg * gebco.TilesY}},
		gopixi.ChannelSet{
			{Name: "ice", Type: gopixi.ChannelInt16},
			{Name: "sub-ice", Type: gopixi.ChannelInt16},
			// specifically, don't need type ID channel in overview
		},
		opts...,
	)

	overviewIterator := gopixi.NewTileOrderWriteIterator(pixiFile, summary.Header, overviewLayer)
	overviewFactor := gebco.GtiffTileSize / *overviewSizeArg
	sample := make(gopixi.Sample, 3)
	err = summary.AppendIterativeLayer(pixiFile, overviewLayer, overviewIterator, func(dstIterator gopixi.IterativeLayerWriter) error {
		for dstIterator.Next() {
			coord := dstIterator.Coordinate()

			// average samples from high res layer
			var iceSum int64
			var subIceSum int64
			var sampleCount int64

			xStart := coord[0] * overviewFactor
			yStart := coord[1] * overviewFactor
			xEnd := (coord[0] + 1) * overviewFactor
			yEnd := (coord[1] + 1) * overviewFactor

			for y := yStart; y < yEnd; y++ {
				for x := xStart; x < xEnd; x++ {
					err := gopixi.SampleInto(readCache, []int{x, y}, sample)
					if err != nil {
						return fmt.Errorf("failed to read sample at coordinate %v: %w", []int{x, y}, err)
					}
					iceSum += int64(sample[0].(int16))
					subIceSum += int64(sample[1].(int16))
					sampleCount += 1
				}
			}

			avgIce := int16(iceSum / sampleCount)
			avgSubIce := int16(subIceSum / sampleCount)

			dstIterator.SetSample(gopixi.Sample{avgIce, avgSubIce})
		}
		return nil
	})

	if err != nil {
		fmt.Printf("failed to write Pixi overview layer: %v\n", err)
		return
	}
}
