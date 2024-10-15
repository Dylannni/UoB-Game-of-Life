package gol

import (
	"strconv"
	"strings"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events     chan<- Event
	ioCommand  chan<- ioCommand
	ioIdle     <-chan bool
	ioFilename chan<- string
	ioOutput   chan<- uint8
	ioInput    <-chan uint8
}

func calculateAliveCells(world [][]byte) []util.Cell {
	var aliveCells []util.Cell

	// Iterate over every cell in the world
	for row := 0; row < len(world); row++ {
		for col := 0; col < len(world[0]); col++ {
			// If the cell is alive (value is 255), add it to the list
			if world[row][col] == 255 {
				aliveCells = append(aliveCells, util.Cell{X: col, Y: row})
			}
		}
	}

	return aliveCells
}

func countLiveNeighbors(world [][]byte, row, col int) int {
	rows := len(world)
	cols := len(world[0])
	neighbors := [8][2]int{
		{-1, -1}, {-1, 0}, {-1, 1}, // Top-left, Top, Top-right
		{0, -1}, {0, 1}, // Left,            Right
		{1, -1}, {1, 0}, {1, 1}, // Bottom-left, Bottom, Bottom-right
	}

	liveNeighbors := 0
	for _, n := range neighbors {
		newRow := (row + n[0] + rows) % rows
		newCol := (col + n[1] + cols) % cols
		if world[newRow][newCol] == 255 {
			liveNeighbors++
		}
	}
	return liveNeighbors
}

func calculateNextState(p Params, world [][]byte) [][]byte {

	newWorld := make([][]byte, p.ImageHeight)
	for i := range newWorld {
		newWorld[i] = make([]byte, p.ImageWidth)
	}

	// Iterate over each cell in the world
	for row := 0; row < p.ImageHeight; row++ {
		for col := 0; col < p.ImageWidth; col++ {
			// Count the live neighbors
			liveNeighbors := countLiveNeighbors(world, row, col)

			// Apply the Game of Life rules
			if world[row][col] == 255 {
				// Cell is alive
				if liveNeighbors < 2 || liveNeighbors > 3 {
					newWorld[row][col] = 0 // Cell dies
				} else {
					newWorld[row][col] = 255 // Cell stays alive
				}
			} else {
				// Cell is dead
				if liveNeighbors == 3 {
					newWorld[row][col] = 255 // Cell becomes alive
				} else {
					newWorld[row][col] = 0 // Cell stays dead
				}
			}
		}
	}
	return newWorld
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	// TODO: Create a 2D slice to store the world.
	world := make([][]byte, p.ImageHeight)
	for i := range world {
		world[i] = make([]byte, p.ImageWidth)

	}
	c.ioCommand <- ioInput
	c.ioFilename <- strings.Join([]string{strconv.Itoa(p.ImageHeight), strconv.Itoa(p.ImageWidth)}, "x")

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			world[y][x] = <-c.ioInput
		}
	}

	turn := 0
	c.events <- StateChange{turn, Executing}

	// TODO: Execute all turns of the Game of Life.

	for turn = 0; turn < p.Turns; turn++ {
		world = calculateNextState(p, world)

		c.events <- TurnComplete{CompletedTurns: turn + 1}
	}

	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: calculateAliveCells(world)}
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
