package main

import (
	"flag"
	"fmt"
	"os"

	"net"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/client/util"
	"uk.ac.bris.cs/gameoflife/stdstruct"
)

type GameOfLife struct{}

// initialise world
func InitWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

// countLiveNeighbors calculates the number of live neighbors for a given cell.
// Parameters:
//   - world: A 2D byte array representing the state of the world, where 255 indicates a live cell, and 0 indicates a dead cell.
//   - row(globalY), col(globalX): The row and column of the current cell to calculate neighbors for.
//   - rows(p.ImageHeight), cols(p.ImageWidth): The total number of rows and columns in the world, used for boundary handling.
//
// Returns:
//   - The number of live neighboring cells.
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

func CalculateNextState(startY, height, width int, extendedWorld [][]byte) []util.Cell {
	var flippedCells []util.Cell
	// Iterate over each cell in the world
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			globalY := y + 1
			globalX := x
			// Count the live neighbors
			liveNeighbors := countLiveNeighbors(extendedWorld, globalY, globalX, len(extendedWorld), len(extendedWorld[0]))
			// Apply the Game of Life rules
			if extendedWorld[globalY][globalX] == 255 && (liveNeighbors < 2 || liveNeighbors > 3) {
				flippedCells = append(flippedCells, util.Cell{X: x, Y: startY + y})
			} else if extendedWorld[globalY][globalX] == 0 && liveNeighbors == 3 {
				flippedCells = append(flippedCells, util.Cell{X: x, Y: startY + y})
			}
		}
	}
	return flippedCells
}

func (s *GameOfLife) CalculateNextTurn(req *stdstruct.SliceRequest, res *stdstruct.SliceResponse) (err error) {

	// world slice with two extra row (one at the top and one at the bottom)
	extendedWorld := req.ExtendedSlice
	height := req.EndY - req.StartY
	width := req.EndX - req.StartX
	flippedCells := CalculateNextState(req.StartY, height, width, extendedWorld)
	res.FlippedCells = flippedCells
	return nil
}

// shutting down the server when k is pressed
func (s *GameOfLife) ShutDown(_ *stdstruct.ShutRequest, _ *stdstruct.ShutResponse) (err error) {
	fmt.Println("Shutting down the server")
	os.Exit(0)
	return nil
}

func main() {
	// Usage: go run controller.go -port XXXX

	// Default port 8080
	pAddr := flag.String("port", "8080", "Port to listen on")
	flag.Parse()
	rpc.Register(&GameOfLife{})
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	fmt.Println("Controller started, listening on port", *pAddr)
	rpc.Accept(listener)
}
