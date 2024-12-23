package gol

import "uk.ac.bris.cs/gameoflife/util"

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

func countLiveNeighbors(world [][]byte, row, col, rows, cols int) int {
	neighbors := [8][2]int{
		{-1, -1}, {-1, 0}, {-1, 1}, // Top-left, Top, Top-right
		{0, -1}, {0, 1}, // Left, Right
		{1, -1}, {1, 0}, {1, 1}, // Bottom-left, Bottom, Bottom-right
	}

	liveNeighbors := 0
	for _, n := range neighbors {
		// Ensures the world wraps around at the edges (i.e. torus-like world)
		newRow := (row + n[0] + rows) % rows
		newCol := (col + n[1] + cols) % cols

		// Example: At a 5x5 world, if the current cell is at (0,0) and the neighbor is {-1, -1} (Top-left),
		// the newRow and newCol would be calculated as:
		// newRow = (0 + (-1) + 5) % 5 = 4  (wraps around to the bottom row)
		// newCol = (0 + (-1) + 5) % 5 = 4  (wraps around to the rightmost column)
		// So, the Top-left neighbor of (0, 0) would be (4, 4), wrapping around from the bottom-right.

		if world[newRow][newCol] == 255 {
			liveNeighbors++
		}
	}
	return liveNeighbors
}

func calculateNextState(startY, endY, startX, endX int, p Params, world [][]byte, c distributorChannels) [][]byte {
	height := endY - startY
	width := endX - startX

	newWorld := make([][]byte, height)
	for i := range newWorld {
		newWorld[i] = make([]byte, width)
	}

	// Iterate over each cell in the world
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {

			globalY := startY + y
			globalX := startX + x
			// Count the live neighbors
			liveNeighbors := countLiveNeighbors(world, globalY, globalX, p.ImageHeight, p.ImageWidth)
			// Apply the Game of Life rules
			if world[globalY][globalX] == 255 {
				// Cell is alive
				if liveNeighbors < 2 || liveNeighbors > 3 {
					newWorld[y][x] = 0 // Cell dies
					c.events <- CellFlipped{CompletedTurns: c.completedTurns, Cell: util.Cell{X: globalX, Y: globalY}}
				} else {
					newWorld[y][x] = 255 // Cell stays alive
				}
			} else {
				// Cell is dead
				if liveNeighbors == 3 {
					newWorld[y][x] = 255 // Cell becomes alive
					c.events <- CellFlipped{CompletedTurns: c.completedTurns, Cell: util.Cell{X: globalX, Y: globalY}}
				} else {
					newWorld[y][x] = 0 // Cell stays dead
				}
			}
		}
	}
	return newWorld
}
