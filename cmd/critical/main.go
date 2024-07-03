package main

import (
	"flag"
	"fmt"
	"os"
	"slices"

	"github.com/owlpinetech/pixi"
)

const (
	CATEGORY_FIELD_ID uint = 0

	CATEGORY_NONE    uint8 = 0
	CATEGORY_SLOPE   uint8 = 1
	CATEGORY_PEAK    uint8 = 2
	CATEGORY_PIT     uint8 = 3
	CATEGORY_SADDLE  uint8 = 4
	CATEGORY_PLATEAU uint8 = 5
)

func main() {
	src := flag.String("src", "", "Path of the image to view a portion of")
	dest := flag.String("dest", "./crit.pixi", "Path of the image that will be created")

	flag.Parse()

	if *src == "" {
		fmt.Println("No image provided, please use -src flag.")
		return
	}

	file, err := os.Open(*src)
	if err != nil {
		fmt.Println("Could not open pixi file:", err)
		return
	}
	defer file.Close()
	outFile, err := os.Create(*dest)
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

	if len(summary.Dimensions) != 2 {
		fmt.Println("Viewable pixi files should only contain two dimensions")
		return
	}

	// read pixi data into image
	readData, err := pixi.ReadAppend(file, summary, 8)
	if err != nil {
		fmt.Println("Failed to open pixi cache reader", err)
		return
	}

	// create categorized gebco map
	critSummary := pixi.Summary{
		Metadata: map[string]string{
			"dimOne": "longitude",
			"dimTwo": "latitude",
		},
		Separated:   false,
		Compression: pixi.CompressionNone,
		Dimensions:  []pixi.Dimension{{Size: 86400, TileSize: 21600 / 4}, {Size: 43200, TileSize: 21600 / 4}},
		Fields:      []pixi.Field{{Name: "category", Type: pixi.FieldUint8}},
	}
	critDataset, err := pixi.NewCacheDataset(critSummary, outFile, 8)
	if err != nil {
		fmt.Println("failed to initialize critical pixi dataset for writing:", err)
		return
	}

	//appendTopo := &pixigebco.GebcoPixiAppend{&readData}
	//cacheTopo := &pixigebco.GebcoPixiCache{critDataset}
	fmt.Println(readData, critDataset)

	/*appendTopo.WalkGet(true, func(x, y int, val int16) bool {
		err := topography.Categorize[int16](appendTopo, cacheTopo, x, y)
		if err != nil {
			fmt.Println("failed to categorize pixel", x, y, err)
			return false
		}
		return true
	})*/

	for x := 0; x < int(critDataset.Dimensions[0].Size); x++ {
		for y := 0; y < int(critDataset.Dimensions[1].Size); y++ {
			fmt.Println("categorizing", x, y)
			//err := topography.Categorize(appendTopo, cacheTopo, x, y)
			//if err != nil {
			//	fmt.Println("failed to categorize pixel", x, y, err)
			//}
			// err := categorize(&readData, critDataset, []uint{uint(x), uint(y)})
			// if err != nil {
			// 	return
			// }
			fmt.Println("categorized pix", x, y)
		}
		fmt.Println("categorized row", x)
	}

	critDataset.Finalize()
}

func categorize(src *pixi.AppendDataset, dst *pixi.CacheDataset, index []uint) error {
	elevVal, err := src.GetSampleField(index, 0)
	if err != nil {
		return err
	}
	elev := elevVal.(int16)
	neighborElevs, _, err := getNeighbors(src, index)
	if err != nil {
		return err
	}

	catVal, err := dst.GetSampleField(index, CATEGORY_FIELD_ID)
	if err != nil {
		return err
	}

	if catVal != CATEGORY_NONE {
		// already set, as part of a prior plateau
		return nil
	} else if slices.Contains(neighborElevs, elev) {
		return categorizePlateau(src, dst, index, elev)
	} else if allHigher(neighborElevs, elev) {
		return dst.SetSampleField(index, CATEGORY_FIELD_ID, CATEGORY_PIT)
	} else if allLower(neighborElevs, elev) {
		return dst.SetSampleField(index, CATEGORY_FIELD_ID, CATEGORY_PEAK)
	} else {
		if neighborSegmentCount(neighborElevs, elev) > 2 {
			return dst.SetSampleField(index, CATEGORY_FIELD_ID, CATEGORY_SADDLE)
		} else {
			return dst.SetSampleField(index, CATEGORY_FIELD_ID, CATEGORY_SLOPE)
		}
	}
}

func getNeighbors(src *pixi.AppendDataset, index []uint) ([]int16, [][]uint, error) {
	neighborElevs := make([]int16, 0)
	neighborIndices := make([][]uint, 0)
	for x := -1; x < 2; x++ {
		for y := -1; y < 2; y++ {
			if x == 0 && y == 0 {
				continue
			}
			xInd := int(index[0]) + x
			yInd := int(index[1]) + y

			// wrap around half the earth for the north pole/south pole
			if yInd < 0 {
				xInd += int(src.Dimensions[0].Size/2) % int(src.Dimensions[0].Size)
				yInd = 0
			} else if yInd >= int(src.Dimensions[1].Size) {
				xInd += int(src.Dimensions[0].Size/2) % int(src.Dimensions[0].Size)
				yInd = int(src.Dimensions[1].Size) - 1
			}

			// wrap around the globe for the meridians
			if xInd < 0 {
				xInd = int(src.Dimensions[0].Size) - 1
			} else if xInd >= int(src.Dimensions[0].Size) {
				xInd = 0
			}

			val, err := src.GetSampleField([]uint{uint(xInd), uint(yInd)}, 0)
			if err != nil {
				return nil, nil, err
			}
			neighborElevs = append(neighborElevs, val.(int16))
			neighborIndices = append(neighborIndices, []uint{uint(xInd), uint(yInd)})
		}
	}
	return neighborElevs, neighborIndices, nil
}

func allHigher(elevs []int16, elev int16) bool {
	for _, e := range elevs {
		if e <= elev {
			return false
		}
	}
	return true
}

func allLower(elevs []int16, elev int16) bool {
	for _, e := range elevs {
		if e >= elev {
			return false
		}
	}
	return true
}

var neighborIndIter []int = []int{1, 2, 4, 7, 6, 5, 3}

func neighborSegmentCount(neighborElevs []int16, elev int16) uint {
	// neighbors are like so, we need to iterate clockwise
	// (-1,-1)  (0, -1)  (1, -1)
	// (-1, 0)            (1, 0)
	// (-1, 1)   (0, 1)   (1, 1)
	startElev := neighborElevs[0]
	startSegType := startElev > elev
	segType := startSegType
	segCount := uint(0)
	for _, neighborInd := range neighborIndIter {
		curSegType := neighborElevs[neighborInd] > elev
		if curSegType != segType {
			segCount += 1
			segType = curSegType
		}
	}
	if segType != startSegType {
		segCount += 1
	}
	return segCount
}

func categorizePlateau(src *pixi.AppendDataset, dst *pixi.CacheDataset, index []uint, elev int16) error {
	plateau, err := extractPlateau(src, index, elev)
	if err != nil {
		return err
	}

	// get the boundary segments of the plateau
	highers, lowers := splitBoundary(plateau, elev)
	higherSegments := segmentizeBoundary(highers)
	lowerSegments := segmentizeBoundary(lowers)
	if len(higherSegments) > 1 || len(lowerSegments) > 1 {
		return setPlatueaBody(dst, plateau, CATEGORY_SADDLE)
	} else if len(higherSegments) == 0 {
		return setPlatueaBody(dst, plateau, CATEGORY_PEAK)
	} else if len(lowerSegments) == 0 {
		return setPlatueaBody(dst, plateau, CATEGORY_PIT)
	} else {
		return setPlatueaBody(dst, plateau, CATEGORY_PLATEAU)
	}
}

func splitBoundary(plateau Plateau, elev int16) ([]Index, []Index) {
	highers := []Index{}
	lowers := []Index{}
	for boundInd, bound := range plateau.boundary {
		if plateau.boundaryElevs[boundInd] > elev {
			highers = append(highers, bound)
		} else {
			lowers = append(lowers, bound)
		}
	}
	return highers, lowers
}

func segmentizeBoundary(boundary []Index) [][]Index {
	segments := [][]Index{}
	for _, bound := range boundary {
		// should we attach this to a previous segment?
		added := false
		for segInd, seg := range segments {
			if neighborManhattanOfAny(seg, bound) {
				segments[segInd] = append(seg, bound)
				added = true
				break
			}
		}
		if !added {
			segments = append(segments, []Index{bound})
		}
	}
	return segments
}

func neighborManhattanOfAny(a []Index, b Index) bool {
	for _, aInd := range a {
		if manhattanNeighbor(aInd, b) {
			return true
		}
	}
	return false
}

func manhattanNeighbor(a Index, b Index) bool {
	if a.x == b.x {
		return a.y == b.y+1 || a.y == b.y-1
	} else if a.y == b.y {
		return a.x == b.x+1 || a.x == b.x-1
	}
	return false
}

func setPlatueaBody(dst *pixi.CacheDataset, plateau Plateau, category uint8) error {
	for _, index := range plateau.body {
		err := dst.SetSampleField([]uint{index.x, index.y}, CATEGORY_FIELD_ID, category)
		if err != nil {
			return err
		}
	}
	return nil
}

type Index struct {
	x uint
	y uint
}

type Plateau struct {
	body          []Index
	boundary      []Index
	boundaryElevs []int16
}

func extractPlateau(src *pixi.AppendDataset, index []uint, elev int16) (Plateau, error) {
	toVisit := []Index{{x: index[0], y: index[1]}}
	plateau := Plateau{[]Index{}, []Index{}, []int16{}}
	for i := 0; i < len(toVisit); i++ {
		plateau.body = append(plateau.body, toVisit[i])
		neighborElevs, neighborIndices, err := getNeighbors(src, index)
		if err != nil {
			return Plateau{}, err
		}

		for nInd, ind := range neighborIndices {
			cmpInd := Index{x: ind[0], y: ind[1]}
			if neighborElevs[nInd] == elev {
				if !slices.Contains(toVisit, cmpInd) {
					toVisit = append(toVisit, cmpInd)
				}
			} else {
				if !slices.Contains(plateau.boundary, cmpInd) {
					plateau.boundary = append(plateau.boundary, cmpInd)
					plateau.boundaryElevs = append(plateau.boundaryElevs, neighborElevs[nInd])
				}
			}
		}
	}
	return plateau, nil
}
