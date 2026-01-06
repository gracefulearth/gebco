package gebco

import (
	"encoding/binary"
	"slices"
	"strconv"
	"testing"

	"github.com/gracefulearth/gopixi"
)

func TestPixiTilesPerGebcoTile(t *testing.T) {
	tests := []struct {
		name                    string
		tileDivisor             int
		expTilesPerGebcoTile    int
		expTilesPerGebcoPerAxis int
	}{
		{
			name:                    "1 Pixi Tile Per Gebco Tile",
			tileDivisor:             1,
			expTilesPerGebcoTile:    1,
			expTilesPerGebcoPerAxis: 1,
		},
		{
			name:                    "4 Pixi Tiles Per Gebco Tile",
			tileDivisor:             2,
			expTilesPerGebcoTile:    4,
			expTilesPerGebcoPerAxis: 2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pixiTileSize := GtiffTileSize / tt.tileDivisor
			iterator := &GebcoTileOrderWriteIterator{
				pixiTilesPerGebcoTilePerAxis: GtiffTileSize / pixiTileSize,
				pixiTilesPerGebcoTile:        (GtiffTileSize / pixiTileSize) * (GtiffTileSize / pixiTileSize),
			}

			if iterator.pixiTilesPerGebcoTile != tt.expTilesPerGebcoTile {
				t.Errorf("pixiTilesPerGebcoTile = %v, want %v", iterator.pixiTilesPerGebcoTile, tt.expTilesPerGebcoTile)
			}
			if iterator.pixiTilesPerGebcoTilePerAxis != tt.expTilesPerGebcoPerAxis {
				t.Errorf("pixiTilesPerGebcoTilePerAxis = %v, want %v", iterator.pixiTilesPerGebcoTilePerAxis, tt.expTilesPerGebcoPerAxis)
			}
		})
	}
}

func TestTileOrder(t *testing.T) {
	testCases := []struct {
		tileDivisor       int
		expectedTileOrder []int
	}{
		{
			tileDivisor:       1,
			expectedTileOrder: []int{0, 1, 2, 3, 4, 5, 6, 7},
		},
		{
			tileDivisor: 2,
			expectedTileOrder: []int{
				0, 1, 8, 9, 2, 3, 10, 11, 4, 5, 12, 13, 6, 7, 14, 15,
				16, 17, 24, 25, 18, 19, 26, 27, 20, 21, 28, 29, 22, 23, 30, 31,
			},
		},
	}

	for _, tt := range testCases {
		t.Run("tile_divisor_"+strconv.Itoa(tt.tileDivisor), func(t *testing.T) {
			pixiTileSize := GtiffTileSize / tt.tileDivisor
			iterator := &GebcoTileOrderWriteIterator{
				backing: nil,
				header:  gopixi.NewHeader(binary.NativeEndian, gopixi.OffsetSize4),
				layer: gopixi.NewLayer(
					"testTile",
					gopixi.DimensionSet{
						{Name: "lng", TileSize: pixiTileSize, Size: TotalWidth},
						{Name: "lat", TileSize: pixiTileSize, Size: TotalHeight},
					},
					gopixi.ChannelSet{
						{Name: "sample", Type: gopixi.ChannelUint16},
					},
				),
				sampleInPixiTile:             -1,
				pixiTilesPerGebcoTilePerAxis: GtiffTileSize / pixiTileSize,
				pixiTilesPerGebcoTile:        (GtiffTileSize / pixiTileSize) * (GtiffTileSize / pixiTileSize),
			}

			var actualTileOrder []int
			for iterator.gebcoTile = range Tiles {
				for iterator.pixiTileInGebco = range iterator.pixiTilesPerGebcoTile {
					actualTileOrder = append(actualTileOrder, iterator.tile())
				}
			}

			if len(actualTileOrder) != len(tt.expectedTileOrder) {
				t.Fatalf("tile order length mismatch: got %d, want %d", len(actualTileOrder), len(tt.expectedTileOrder))
			}

			if !slices.Equal(actualTileOrder, tt.expectedTileOrder) {
				t.Errorf("tile order mismatch: got %v, want %v", actualTileOrder, tt.expectedTileOrder)
			}
		})
	}
}
