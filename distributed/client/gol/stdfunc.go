package gol

import (
	"strconv"
	"strings"

	"uk.ac.bris.cs/gameoflife/client/util"
)

// initialise world
func initWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

func calculateAliveCells(p Params, world [][]byte) []util.Cell {
	var aliveCells []util.Cell

	// Iterate over every cell in the world
	for row := 0; row < p.ImageHeight; row++ {
		for col := 0; col < p.ImageWidth; col++ {
			// If the cell is alive (value is 255), add it to the list
			if world[row][col] == 255 {
				aliveCells = append(aliveCells, util.Cell{X: col, Y: row})
			}
		}
	}
	return aliveCells
}

// send the world into output
func outputImage(c distributorChannels, p Params, world [][]byte) {
	c.ioCommand <- ioOutput
	filename := strings.Join([]string{strconv.Itoa(p.ImageHeight), strconv.Itoa(p.ImageWidth), strconv.Itoa(c.completedTurns)}, "x")
	c.ioFilename <- filename
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.ioOutput <- world[y][x]
		}
	}
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle
	c.events <- ImageOutputComplete{c.completedTurns, filename}

}
