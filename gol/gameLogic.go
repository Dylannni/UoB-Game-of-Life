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
		newRow := (row + n[0] + rows) % rows
		newCol := (col + n[1] + cols) % cols
		// 调用 world 函数来获取细胞状态
		if world[newRow][newCol] == 255 {
			liveNeighbors++
		}
	}
	return liveNeighbors
}

func calculateNextState(startY, endY, startX, endX int, p Params, world [][]byte, c distributorChannels) [][]byte {
	// func calculateNextState(startY, endY, startX, endX int, p Params, world func(y, x int) byte, c distributorChannels) [][]byte {

	height := endY - startY
	width := endX - startX

	newWorld := make([][]byte, height)
	for i := range newWorld {
		newWorld[i] = make([]byte, width)
	}

	// Iterate over each cell in the world
	for row := 0; row < height; row++ {
		for col := 0; col < width; col++ {

			globalRow := startY + row
			globalCol := startX + col
			// Count the live neighbors
			liveNeighbors := countLiveNeighbors(world, globalRow, globalCol, p.ImageHeight, p.ImageWidth)
			// Apply the Game of Life rules
			if world[globalRow][globalCol] == 255 {
				// Cell is alive
				if liveNeighbors < 2 || liveNeighbors > 3 {
					newWorld[row][col] = 0 // Cell dies
					c.events <- CellFlipped{CompletedTurns: c.completedTurns, Cell: util.Cell{X: globalRow, Y: globalCol}}
				} else {
					newWorld[row][col] = 255 // Cell stays alive
				}
			} else {
				// Cell is dead
				if liveNeighbors == 3 {
					newWorld[row][col] = 255 // Cell becomes alive
					c.events <- CellFlipped{CompletedTurns: c.completedTurns, Cell: util.Cell{X: globalRow, Y: globalCol}}
				} else {
					newWorld[row][col] = 0 // Cell stays dead
				}
			}
		}
	}
	return newWorld
}
