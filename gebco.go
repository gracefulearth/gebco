package gebco

import (
	"fmt"
	"image"
	"math"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/gracefulearth/image/tiff"
)

const (
	GtiffTileSize int = 21600                         // The number of pixels across a strip of longitude/latitude in a single GEBCO tif tile.
	GtiffSize     int = GtiffTileSize * GtiffTileSize // The total number of pixels in a single GEBCO tif tile.

	TilesY int = 2               // The number of tiles in the Y (latitude) direction in the GEBCO dataset.
	TilesX int = 4               // The number of tiles in the X (longitude) direction in the GEBCO dataset.
	Tiles  int = TilesX * TilesY // The total number of tif tiles in the GEBCO dataset.

	TotalWidth  int = TilesX * GtiffTileSize   // The number of pixels across a strip of latitude in the GEBCO dataset.
	TotalHeight int = TilesY * GtiffTileSize   // The number of pixels along a strip of longitude in the GEBCO dataset.
	TotalPixels     = TotalWidth * TotalHeight // The total number of pixels in the GEBCO dataset.

	ArcSecIncrement int = 15                   // The number of arc seconds each pixel is spaced apart from each neighboring pixel
	PixelsPerMinute     = 60 / ArcSecIncrement // The number of GEBCO pixels in a single arc minute.
	PixelsPerDegree     = 60 * PixelsPerMinute // The number of GEBCO pixels in a single degree.
)

type GebcoDataType byte

const (
	GebcoDataIce    GebcoDataType = iota // Bathymetric data including surface ice cover (e.g., ice shelves, grounded ice, sea ice).
	GebcoDataSubIce                      // Bathymetric data representing the seafloor beneath ice cover, excluding surface ice features.
	GebcoDataTypeId                      // Source type of a GEBCO depth value.
)

// fileString returns the string representation of the GebcoDataType for use in file names.
func (g GebcoDataType) fileString() string {
	switch g {
	case GebcoDataIce:
		return ""
	case GebcoDataSubIce:
		return "_sub_ice"
	case GebcoDataTypeId:
		return "_tid"
	default:
		panic("unknown GebcoDataType")
	}
}

// GebcoTypeId represents the source type of a GEBCO depth value.
type GebcoTypeId byte

const (
	GebcoTypeLand GebcoTypeId = 0 // Value represents land area (no depth) from SRTM15+ V2.7 dataset.

	// Direct Measurements

	GebcoTypeSingleBeam  GebcoTypeId = 10 // Depth value collected by a single-beam echo sounder.
	GebcoTypeMultiBeam   GebcoTypeId = 11 // Depth value collected by a multi-beam echo sounder.
	GebcoTypeSeismic     GebcoTypeId = 12 // Depth value collected by seismic methods.
	GebcoTypeIsolated    GebcoTypeId = 13 // Depth value collected by isolated soundings, not part of a systematic survey or track.
	GebcoTypeEncSounding GebcoTypeId = 14 // Depth value extracted from an Electronic Navigation Chart.
	GebcoTypeLidar       GebcoTypeId = 15 // Depth value derived from bathymetric LIDAR sensor.
	GebcoTypeOptical     GebcoTypeId = 16 // Depth value derived from optical light sensor.
	GebcoTypeCombination GebcoTypeId = 17 // Depth value derived from a combination of direct measurement methods.

	// Indirect Measurements

	GebcoTypeSatelliteGravity            GebcoTypeId = 40 // Depth value dervied from interpolated data guided by satellite-derived gravity measurements.
	GebcoTypeInterpolated                GebcoTypeId = 41 // Depth value derived from interpolation using a computer algorithm.
	GebcoTypeContour                     GebcoTypeId = 42 // Depth value derived from digitized contour lines.
	GebcoTypeEncContour                  GebcoTypeId = 43 // Depth value derived from contour lines extracted from an Electronic Navigation Chart.
	GebcoTypeMultisourceSatelliteGravity GebcoTypeId = 44 // Depth value derived from multisource data guided by satellite-derived gravity measurements.
	GebcoTypeFlightGravity               GebcoTypeId = 45 // Depth value derived from multisource data guided by airborne gravity measurements.
	GebcoTypeIcebergDraft                GebcoTypeId = 46 // Depth value derived from grounded iceberg draft measurements, using satellite-dervied freeboard measurements.
	GebcoTypeArgoDrift                   GebcoTypeId = 47 // Depth value derived from Argo float drift measurements.

	// Unknown

	GebcoTypePregenerated GebcoTypeId = 70 // Depth value from pregenerated bathymetry grid data source derived from mixed sources.
	GebcoTypeUnknown      GebcoTypeId = 71 // Depth value from unknown data source.
	GebcoTypeSteering     GebcoTypeId = 72 // Depth value used to constrain the grid in areas of poor data coverage.
)

type GebcoTifFile struct {
	x, y int
	year int
	data GebcoDataType
}

func (g GebcoTifFile) String() string {
	return fmt.Sprintf("GEBCO(%d%s,n%d,s%d,w%d,e%d)", g.year, g.data.fileString(), g.North(), g.South(), g.West(), g.East())
}

func (g GebcoTifFile) North() int {
	return 90 - g.y*90
}

func (g GebcoTifFile) South() int {
	return g.North() - 90
}

func (g GebcoTifFile) West() int {
	return -180 + g.x*90
}

func (g GebcoTifFile) East() int {
	return g.West() + 90
}

func (g GebcoTifFile) FileName() string {
	return fmt.Sprintf("gebco_%d%s_n%d.0_s%d.0_w%d.0_e%d.0.tif", g.year, g.data.fileString(), g.North(), g.South(), g.West(), g.East())
}

func (g GebcoTifFile) Load(folder string) (image.Image, error) {
	path := filepath.Join(folder, g.FileName())
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

func (g *GebcoTifFile) UnmarshalText(text []byte) error {
	strText := string(text)

	// determine data type
	if strings.Contains(strText, "_sub_ice") {
		g.data = GebcoDataSubIce
		strText = strings.ReplaceAll(strText, "_sub_ice", "")
	} else if strings.Contains(strText, "_tid") {
		g.data = GebcoDataTypeId
		strText = strings.ReplaceAll(strText, "_tid", "")
	} else {
		g.data = GebcoDataIce
	}

	var year, north, south, west, east int
	fmt.Sscanf(strText, "gebco_%d_n%d.0_s%d.0_w%d.0_e%d.0.tif", &year, &north, &south, &west, &east)
	g.year = year
	g.x = (west + 180) / 90
	g.y = (90 - north) / 90

	return nil
}

func GebcoTiles(year int, dataType GebcoDataType) []GebcoTifFile {
	tiles := make([]GebcoTifFile, 0, TilesX*TilesY)
	for y := range TilesY {
		for x := range TilesX {
			tiles = append(tiles, GebcoTifFile{
				x:    x,
				y:    y,
				year: year,
				data: dataType,
			})
		}
	}
	return tiles
}

type GebcoTifLayer struct {
	Ice    GebcoTifFile
	SubIce GebcoTifFile
	Tid    GebcoTifFile
}

func (layer GebcoTifLayer) Load(folder string) (ice, subIce, tid image.Image, err error) {
	var iceErr, subIceErr, tidErr error

	wg := sync.WaitGroup{}
	wg.Go(func() {
		ice, iceErr = layer.Ice.Load(folder)
	})
	wg.Go(func() {
		subIce, subIceErr = layer.SubIce.Load(folder)
	})
	wg.Go(func() {
		tid, tidErr = layer.Tid.Load(folder)
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

func GebcoLayeredTiles(year int) []GebcoTifLayer {
	tiles := make([]GebcoTifLayer, 0, TilesX*TilesY)
	for y := range TilesY {
		for x := range TilesX {
			tiles = append(tiles, GebcoTifLayer{
				Ice: GebcoTifFile{
					x:    x,
					y:    y,
					year: year,
					data: GebcoDataIce,
				},
				SubIce: GebcoTifFile{
					x:    x,
					y:    y,
					year: year,
					data: GebcoDataSubIce,
				},
				Tid: GebcoTifFile{
					x:    x,
					y:    y,
					year: year,
					data: GebcoDataTypeId,
				},
			})
		}
	}
	return tiles
}

func CheckDirectoryComplete(folder string, layeredTiles []GebcoTifLayer) []string {
	missingFiles := []string{}
	for _, tile := range layeredTiles {
		if _, err := os.Stat(filepath.Join(folder, tile.Ice.FileName())); os.IsNotExist(err) {
			missingFiles = append(missingFiles, tile.Ice.FileName())
		}
		if _, err := os.Stat(filepath.Join(folder, tile.SubIce.FileName())); os.IsNotExist(err) {
			missingFiles = append(missingFiles, tile.SubIce.FileName())
		}
		if _, err := os.Stat(filepath.Join(folder, tile.Tid.FileName())); os.IsNotExist(err) {
			missingFiles = append(missingFiles, tile.Tid.FileName())
		}
	}
	return missingFiles
}

type GebcoArc struct {
	Degree int
	Minute int
	Second int
}

func FromDecimal(d float64) GebcoArc {
	minutes := (math.Abs(d) - math.Floor(math.Abs(d))) * 60.0
	if int(d) == 0 {
		minutes = math.Copysign(minutes, d)
	}
	seconds := (math.Abs(minutes) - math.Floor(math.Abs(minutes))) * 60.0
	if int(minutes) == 0 {
		seconds = math.Copysign(seconds, minutes)
	}
	return GebcoArc{
		Degree: int(d),
		Minute: int(minutes),
		Second: int(seconds),
	}
}

func (g GebcoArc) String() string {
	return fmt.Sprintf("%d°%d'%d\"", g.Degree, g.Minute, g.Second)
}

func (g GebcoArc) ToDecimal() float64 {
	return float64(g.Degree) + (float64(g.Minute) / 60) + (float64(g.Second) / (3600))
}

func (a GebcoArc) Equal(b GebcoArc) bool {
	return a.Degree == b.Degree && a.Minute == b.Minute && a.Second == b.Second
}
