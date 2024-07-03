package pixigebco

import (
	"fmt"
	"math"
)

const (
	ArcSecIncrement int = 15 // The number of arc seconds each pixel is spaced apart from each neighboring pixel

	TotalWidth  int = 86400                    // The number of pixels across a strip of latitude in the GEBCO dataset. 86400 pixels.
	TotalHeight int = 43200                    // The number of pixels along a strip of longitude in the GEBCO dataset. 43200 pixels.
	TotalPixels     = TotalWidth * TotalHeight // The total number of pixels in the GEBCO dataset.

	PixelsPerMinute = 60 / ArcSecIncrement
	PixelsPerDegree = 60 * PixelsPerMinute
)

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

type GebcoTile struct {
	FileName   string
	NorthStart GebcoArc
	WestStart  GebcoArc
}

func (g GebcoTile) String() string {
	return fmt.Sprintf("gebco_tile_north(%s)_west(%s) - %s", g.NorthStart, g.WestStart, g.FileName)
}

/*type GebcoPixiCache struct {
	*pixi.CacheDataset
}

func (g *GebcoPixiCache) GetValue(x, y int) topography.CriticalPointKind {
	val, err := g.GetSampleField([]uint{uint(x), uint(y)}, 0)
	if err != nil {
		panic(err)
	}
	return topography.CriticalPointKind(val.(uint8))
}

func (g *GebcoPixiCache) SetValue(x, y int, val topography.CriticalPointKind) {
	err := g.SetSampleField([]uint{uint(x), uint(y)}, 0, uint8(val))
	if err != nil {
		panic(err)
	}
}

func (r *GebcoPixiCache) WalkGet(rowMajor bool, visitor func(x int, y int, val topography.CriticalPointKind) bool) {
	if rowMajor {
		for y := 0; y < int(r.Dimensions[1].Size); y++ {
			for x := 0; x < int(r.Dimensions[0].Size); x++ {
				val, err := r.GetSampleField([]uint{uint(x), uint(y)}, 0)
				if err != nil {
					panic(err)
				}
				if !visitor(x, y, topography.CriticalPointKind(val.(uint8))) {
					return
				}
			}
		}
	} else {
		for x := 0; x < int(r.Dimensions[0].Size); x++ {
			for y := 0; y < int(r.Dimensions[1].Size); y++ {
				val, err := r.GetSampleField([]uint{uint(x), uint(y)}, 0)
				if err != nil {
					panic(err)
				}
				if !visitor(x, y, topography.CriticalPointKind(val.(uint8))) {
					return
				}
			}
		}
	}
}

func (r *GebcoPixiCache) WalkSet(rowMajor bool, visitor func(x int, y int, val topography.CriticalPointKind) (topography.CriticalPointKind, bool)) {
	if rowMajor {
		for y := 0; y < int(r.Dimensions[1].Size); y++ {
			for x := 0; x < int(r.Dimensions[0].Size); x++ {
				val, err := r.GetSampleField([]uint{uint(x), uint(y)}, 0)
				if err != nil {
					panic(err)
				}
				newVal, cont := visitor(x, y, topography.CriticalPointKind(val.(uint8)))
				err = r.SetSampleField([]uint{uint(x), uint(y)}, 0, uint8(newVal))
				if err != nil {
					panic(err)
				}
				if !cont {
					return
				}
			}
		}
	} else {
		for x := 0; x < int(r.Dimensions[0].Size); x++ {
			for y := 0; y < int(r.Dimensions[1].Size); y++ {
				val, err := r.GetSampleField([]uint{uint(x), uint(y)}, 0)
				if err != nil {
					panic(err)
				}
				newVal, cont := visitor(x, y, topography.CriticalPointKind(val.(uint8)))
				err = r.SetSampleField([]uint{uint(x), uint(y)}, 0, uint8(newVal))
				if err != nil {
					panic(err)
				}
				if !cont {
					return
				}
			}
		}
	}
}

func (r *GebcoPixiCache) WalkNeighbors(x int, y int, n topography.Neighborhood, visitor func(x int, y int, val topography.CriticalPointKind)) {
	n.WalkNeighbors(x, y, func(xN, yN int) {
		if xN >= 0 && xN < int(r.Dimensions[0].Size) && yN >= 0 && yN < int(r.Dimensions[1].Size) {
			val, err := r.GetSampleField([]uint{uint(xN), uint(yN)}, 0)
			if err != nil {
				panic(err)
			}
			visitor(xN, yN, topography.CriticalPointKind(val.(uint8)))
		}
	})
}

type GebcoPixiAppend struct {
	*pixi.AppendDataset
}

func (g *GebcoPixiAppend) GetValue(x, y int) int16 {
	val, err := g.GetSampleField([]uint{uint(x), uint(y)}, 0)
	if err != nil {
		panic(err)
	}
	return val.(int16)
}

func (g *GebcoPixiAppend) SetValue(x, y int, val int16) {
	err := g.SetSampleField([]uint{uint(x), uint(y)}, 0, val)
	if err != nil {
		panic(err)
	}
}

func (r *GebcoPixiAppend) WalkGet(rowMajor bool, visitor func(x int, y int, val int16) bool) {
	if rowMajor {
		for y := 0; y < int(r.Dimensions[1].Size); y++ {
			for x := 0; x < int(r.Dimensions[0].Size); x++ {
				val, err := r.GetSampleField([]uint{uint(x), uint(y)}, 0)
				if err != nil {
					panic(err)
				}
				if !visitor(x, y, val.(int16)) {
					return
				}
			}
		}
	} else {
		for x := 0; x < int(r.Dimensions[0].Size); x++ {
			for y := 0; y < int(r.Dimensions[1].Size); y++ {
				val, err := r.GetSampleField([]uint{uint(x), uint(y)}, 0)
				if err != nil {
					panic(err)
				}
				if !visitor(x, y, val.(int16)) {
					return
				}
			}
		}
	}
}

func (r *GebcoPixiAppend) WalkSet(rowMajor bool, visitor func(x int, y int, val int16) (int16, bool)) {
	if rowMajor {
		for y := 0; y < int(r.Dimensions[1].Size); y++ {
			for x := 0; x < int(r.Dimensions[0].Size); x++ {
				val, err := r.GetSampleField([]uint{uint(x), uint(y)}, 0)
				if err != nil {
					panic(err)
				}
				newVal, cont := visitor(x, y, val.(int16))
				err = r.SetSampleField([]uint{uint(x), uint(y)}, 0, newVal)
				if err != nil {
					panic(err)
				}
				if !cont {
					return
				}
			}
		}
	} else {
		for x := 0; x < int(r.Dimensions[0].Size); x++ {
			for y := 0; y < int(r.Dimensions[1].Size); y++ {
				val, err := r.GetSampleField([]uint{uint(x), uint(y)}, 0)
				if err != nil {
					panic(err)
				}
				newVal, cont := visitor(x, y, val.(int16))
				err = r.SetSampleField([]uint{uint(x), uint(y)}, 0, newVal)
				if err != nil {
					panic(err)
				}
				if !cont {
					return
				}
			}
		}
	}
}

func (r *GebcoPixiAppend) WalkNeighbors(x int, y int, n topography.Neighborhood, visitor func(x int, y int, val int16)) {
	n.WalkNeighbors(x, y, func(xN, yN int) {
		// wrap around half the earth for the north pole/south pole
		if yN < 0 {
			xN += int(r.Dimensions[0].Size/2) % int(r.Dimensions[0].Size)
			yN = 0
		} else if yN >= int(r.Dimensions[1].Size) {
			xN += int(r.Dimensions[0].Size/2) % int(r.Dimensions[0].Size)
			yN = int(r.Dimensions[1].Size) - 1
		}

		// wrap around the globe for the meridians
		if xN < 0 {
			xN = int(r.Dimensions[0].Size) - 1
		} else if x >= int(r.Dimensions[0].Size) {
			xN = 0
		}
		val, err := r.GetSampleField([]uint{uint(xN), uint(yN)}, 0)
		if err != nil {
			panic(err)
		}
		visitor(xN, yN, val.(int16))
	})
}
*/
