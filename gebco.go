package pixigebco

import (
	"fmt"
	"math"
)

const (
	ArcSecIncrement int = 15 // The number of arc seconds each pixel is spaced apart from each neighboring pixel
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
