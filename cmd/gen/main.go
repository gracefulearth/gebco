package main

import (
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/owlpinetech/pixi"
	"github.com/rngoodner/gtiff"
)

const (
	ArcSecIncrement int = 15 // The number of arc seconds each pixel is spaced apart from each neighboring pixel
)

type GebcoArc struct {
	Degree int
	Minute int
	Second int
}

func (g GebcoArc) String() string {
	return fmt.Sprintf("%d°%d'%d\"", g.Degree, g.Minute, g.Second)
}

func (g GebcoArc) ToDecimal() float64 {
	return float64(g.Degree) + (float64(g.Minute) / 60) + (float64(g.Second) / (3600))
}

type GebcoTile struct {
	fileName   string
	northStart GebcoArc
	westStart  GebcoArc
}

func (g GebcoTile) String() string {
	return fmt.Sprintf("gebco_tile_north(%s)_west(%s) - %s", g.northStart, g.westStart, g.fileName)
}

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

	tiles := []GebcoTile{}
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
			tile := GebcoTile{
				fileName:   file.Name(),
				northStart: GebcoArc{Degree: int(northDeg)},
				westStart:  GebcoArc{Degree: int(westDeg)},
			}
			tiles = append(tiles, tile)
			fmt.Println(tile)
		}
	}

	// open append pixi file
	gebcoSummary := pixi.Summary{
		Metadata: map[string]string{
			"dimOne": "longitude",
			"dimTwo": "latitude",
		},
		Datasets: []pixi.DataSet{
			{
				Separated:   false,
				Compression: pixi.CompressionGzip,
				Dimensions:  []pixi.Dimension{{Size: 86400, TileSize: 21600}, {Size: 43200, TileSize: 21600}},
				Fields:      []pixi.Field{{Name: "elevation", Type: pixi.FieldInt16}},
			},
		},
	}
	file, err := os.Create(*pixiPath)
	if err != nil {
		fmt.Println("failed to open output pixi file for writing:", err)
		return
	}
	defer file.Close()

	err = pixi.WriteSummary(file, gebcoSummary)
	if err != nil {
		fmt.Println("failed to write gebco pixi summary:", err)
		return
	}
	offset, err := file.Seek(0, io.SeekCurrent)
	if err != nil {
		fmt.Println("failed to seek to start of data:", err)
		return
	}

	gebcoAppend, err := pixi.NewAppendDataset(gebcoSummary.Datasets[0], file, 4, offset)
	if err != nil {
		fmt.Println("failed to initialize gebco pixi dataset for writing:", err)
		return
	}

	// read geotiff and write to pixi
	for _, geotiff := range tiles {
		gFile, err := os.Open(filepath.Join(*tiffsPath, geotiff.fileName))
		if err != nil {
			fmt.Println("failed to open geotiff file:", err)
			return
		}

		tags, header, err := gtiff.ReadTags(gFile)
		if err != nil {
			fmt.Println("failed to read geotiff tags:", err)
			return
		}

		tileX := (geotiff.westStart.Degree + 180) / 90
		tileY := geotiff.northStart.Degree / 90
		fmt.Println("x, y -> ps -> comp -> photo -> w, h", tileX, tileY, tags.BitsPerSample, tags.Compression, tags.PhotometricInterpretation, tags.ImageWidth, tags.ImageLength)
		data, _ := gtiff.ReadData16(gFile, header, tags)
		fmt.Println("data size:", len(data))
		for i, e := range data {
			x := i%int(tags.ImageWidth) + (tileX * int(tags.ImageWidth))
			y := i/int(tags.ImageLength) + (tileY * int(tags.ImageLength))
			gebcoAppend.SetSampleField([]uint{uint(x), uint(y)}, 0, int16(e))
		}
	}
}
