package gebco

import (
	"io"
	"sync"

	"github.com/gracefulearth/gopixi"
)

const (
	nonSeparatedKey = -1
)

type tileWriteCommand struct {
	tileIndex int
	tiles     map[int][]byte
}

// GebcoTileOrderWriteIterator implements gopixi.IterativeLayerWriter writing tiles in GEBCO tiff tile order.
// This is so we only have to load one GEBCO tile at a time when building from GEBCO tiff files. This particular
// iterator requires the Pixi layer to have a tile size that is a divisor of the GEBCO tile size (21600x21600). It
// also assumes the layer dimensions are ordered x then y (i.e. row-major order).
type GebcoTileOrderWriteIterator struct {
	backing                      io.WriteSeeker
	header                       gopixi.Header
	layer                        gopixi.Layer
	pixiTilesPerGebcoTilePerAxis int
	pixiTilesPerGebcoTile        int

	sampleInPixiTile int // the index of the current sample within the current Pixi tile
	pixiTileInGebco  int // the index of this Pixi tile within the current GEBCO tile
	gebcoTile        int // the index of the current 21600x21600 GEBCO tile being read from

	wg           sync.WaitGroup
	writeLock    sync.RWMutex
	writeQueue   chan tileWriteCommand
	currentError error

	tiles map[int][]byte
}

var _ gopixi.IterativeLayerWriter = (*gopixi.TileOrderWriteIterator)(nil)

func NewGebcoTileOrderWriteIterator(backing io.WriteSeeker, header gopixi.Header, layer gopixi.Layer) *GebcoTileOrderWriteIterator {
	tilesPerGebcoPerAxis := GtiffTileSize / layer.Dimensions[0].TileSize

	iterator := &GebcoTileOrderWriteIterator{
		backing: backing,
		header:  header,
		layer:   layer,

		sampleInPixiTile: -1, // so first Next() goes to 0

		writeQueue: make(chan tileWriteCommand, 100),

		tiles:                        make(map[int][]byte),
		pixiTilesPerGebcoTile:        tilesPerGebcoPerAxis * tilesPerGebcoPerAxis,
		pixiTilesPerGebcoTilePerAxis: tilesPerGebcoPerAxis,
	}

	if layer.Separated {
		for channelIndex := range layer.Channels {
			tileSize := layer.DiskTileSize(layer.Dimensions.Tiles() * channelIndex)
			iterator.tiles[channelIndex] = make([]byte, tileSize)
		}
	} else {
		tileSize := layer.DiskTileSize(0)
		iterator.tiles[nonSeparatedKey] = make([]byte, tileSize)
	}

	iterator.wg.Go(func() {
		for tileWrites := range iterator.writeQueue {
			err := iterator.writeTiles(tileWrites.tiles, tileWrites.tileIndex)
			if err != nil {
				iterator.writeLock.Lock()
				iterator.currentError = err
				iterator.writeLock.Unlock()
				return
			}
		}
	})

	return iterator
}

func (t *GebcoTileOrderWriteIterator) Layer() gopixi.Layer {
	return t.layer
}

func (t *GebcoTileOrderWriteIterator) Done() {
	close(t.writeQueue)
	t.wg.Wait()
}

func (t *GebcoTileOrderWriteIterator) Error() error {
	t.writeLock.RLock()
	defer t.writeLock.RUnlock()
	return t.currentError
}

func (t *GebcoTileOrderWriteIterator) tile() int {
	xGebco := t.gebcoTile % TilesX
	yGebco := t.gebcoTile / TilesX

	yInGebco := t.pixiTileInGebco / t.pixiTilesPerGebcoTilePerAxis
	xInGebco := t.pixiTileInGebco % t.pixiTilesPerGebcoTilePerAxis

	xTile := xGebco*t.pixiTilesPerGebcoTilePerAxis + xInGebco
	yTile := yGebco*t.pixiTilesPerGebcoTilePerAxis + yInGebco
	return yTile*t.layer.Dimensions[0].Tiles() + xTile
}

func (t *GebcoTileOrderWriteIterator) Next() bool {
	if t.Error() != nil {
		return false
	}

	t.sampleInPixiTile += 1
	if t.sampleInPixiTile >= t.layer.Dimensions.TileSamples() {
		writeTile := t.tile()
		t.sampleInPixiTile = 0
		t.pixiTileInGebco += 1
		if t.pixiTileInGebco >= t.pixiTilesPerGebcoTile {
			t.pixiTileInGebco = 0
			t.gebcoTile += 1
		}

		t.writeQueue <- tileWriteCommand{tiles: t.tiles, tileIndex: writeTile}
		t.tiles = make(map[int][]byte)

		// check if we are done
		if t.tile() >= t.layer.Dimensions.Tiles() {
			return false
		} else {
			if t.layer.Separated {
				for channelIndex := range t.layer.Channels {
					tileSize := t.layer.DiskTileSize(t.tile() + t.layer.Dimensions.Tiles()*channelIndex)
					t.tiles[channelIndex] = make([]byte, tileSize)
				}
			} else {
				tileSize := t.layer.DiskTileSize(t.tile())
				t.tiles[nonSeparatedKey] = make([]byte, tileSize)
			}
		}
	}

	return true
}

func (t *GebcoTileOrderWriteIterator) Coordinate() gopixi.SampleCoordinate {
	tileSelector := gopixi.TileSelector{
		Tile:   t.tile(),
		InTile: t.sampleInPixiTile,
	}
	return tileSelector.
		ToTileCoordinate(t.layer.Dimensions).
		ToSampleCoordinate(t.layer.Dimensions)
}

func (t *GebcoTileOrderWriteIterator) SetChannel(channelIndex int, value any) {
	if t.Error() != nil {
		return
	}

	// Update Min/Max for the channel
	t.layer.Channels[channelIndex] = t.layer.Channels[channelIndex].WithMinMax(value)

	if t.layer.Separated {
		tileData := t.tiles[channelIndex]
		if t.layer.Channels[channelIndex].Type == gopixi.ChannelBool {
			gopixi.PackBool(value.(bool), tileData, t.sampleInPixiTile)
		} else {
			inTileOffset := t.sampleInPixiTile * t.layer.Channels[channelIndex].Size()
			t.layer.Channels[channelIndex].PutValue(value, t.header.ByteOrder, tileData[inTileOffset:])
		}
	} else {
		tileData := t.tiles[nonSeparatedKey]
		inTileOffset := t.sampleInPixiTile * t.layer.Channels.Size()
		channelOffset := t.layer.Channels.Offset(channelIndex)
		t.layer.Channels[channelIndex].PutValue(value, t.header.ByteOrder, tileData[inTileOffset+channelOffset:])
	}
}

func (t *GebcoTileOrderWriteIterator) SetSample(value gopixi.Sample) {
	if t.Error() != nil {
		return
	}

	// Update Min/Max for all channels in the sample
	for channelIndex, channelValue := range value {
		t.layer.Channels[channelIndex] = t.layer.Channels[channelIndex].WithMinMax(channelValue)
	}

	if t.layer.Separated {
		for channelIndex, channel := range t.layer.Channels {
			tileData := t.tiles[channelIndex]
			if channel.Type == gopixi.ChannelBool {
				gopixi.PackBool(value[channelIndex].(bool), tileData, t.sampleInPixiTile)
			} else {
				inTileOffset := t.sampleInPixiTile * channel.Size()
				channel.PutValue(value[channelIndex], t.header.ByteOrder, tileData[inTileOffset:])
			}
		}
	} else {
		tileData := t.tiles[nonSeparatedKey]
		inTileOffset := t.sampleInPixiTile * t.layer.Channels.Size()
		for channelIndex, channel := range t.layer.Channels {
			channel.PutValue(value[channelIndex], t.header.ByteOrder, tileData[inTileOffset:])
			inTileOffset += channel.Size()
		}
	}
}

func (t *GebcoTileOrderWriteIterator) writeTiles(tiles map[int][]byte, tileIndex int) error {
	if t.layer.Separated {
		for channelIndex := range t.layer.Channels {
			channelTile := tileIndex + t.layer.Dimensions.Tiles()*channelIndex
			err := t.layer.WriteTile(t.backing, t.header, channelTile, tiles[channelIndex])
			if err != nil {
				return err
			}
		}
		return nil
	} else {
		return t.layer.WriteTile(t.backing, t.header, tileIndex, tiles[nonSeparatedKey])
	}
}
