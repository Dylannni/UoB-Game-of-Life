package gol

import (
	"fmt"
	"net/rpc"
	"strconv"
	"strings"
	"time"

	"uk.ac.bris.cs/gameoflife/client/util"
	"uk.ac.bris.cs/gameoflife/stdstruct"
)

type distributorChannels struct {
	events         chan<- Event
	ioCommand      chan<- ioCommand
	ioIdle         <-chan bool
	ioFilename     chan<- string
	ioOutput       chan<- uint8
	ioInput        <-chan uint8
	completedTurns int
	keyPresses     <-chan rune
}

// initialise world
func InitWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

// func calculateAliveCells(p Params, world [][]byte) []stdstruct.Cell {
// 	var aliveCells []stdstruct.Cell

// 	// Iterate over every cell in the world
// 	for row := 0; row < p.ImageHeight; row++ {
// 		for col := 0; col < p.ImageWidth; col++ {
// 			// If the cell is alive (value is 255), add it to the list
// 			if world[row][col] == 255 {
// 				aliveCells = append(aliveCells, stdstruct.Cell{X: col, Y: row})
// 			}
// 		}
// 	}
// 	return aliveCells
// }

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

func CalculateNextState(startY, endY, startX, endX int, p Params, world [][]byte) [][]byte {
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
					//c.events <- CellFlipped{CompletedTurns: c.completedTurns, Cell: stdstruct.Cell{X: globalX, Y: globalY}}
				} else {
					newWorld[y][x] = 255 // Cell stays alive
				}
			} else {
				// Cell is dead
				if liveNeighbors == 3 {
					newWorld[y][x] = 255 // Cell becomes alive
					//c.events <- CellFlipped{CompletedTurns: c.completedTurns, Cell: stdstruct.Cell{X: globalX, Y: globalY}}
				} else {
					newWorld[y][x] = 0 // Cell stays dead
				}
			}
		}
	}
	return newWorld
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	client, err := rpc.Dial("tcp", "44.203.99.239:8030")

	if err != nil {
		fmt.Println("Error connecting to server:", err)
		return
	}
	defer client.Close()

	// TODO: Create a 2D slice to store the world.
	world := initWorld(p.ImageHeight, p.ImageWidth)
	ticker := time.NewTicker(2 * time.Second)

	c.ioCommand <- ioInput
	c.ioFilename <- strings.Join([]string{strconv.Itoa(p.ImageHeight), strconv.Itoa(p.ImageWidth)}, "x")
	// add value to the input
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			val := <-c.ioInput
			world[y][x] = val
			if val == 255 {
				c.events <- CellFlipped{CompletedTurns: 0, Cell: util.Cell{X: x, Y: y}}
			}
		}
	}

	turn := 0
	c.events <- StateChange{turn, Executing}

	// TODO: Execute all turns of the Game of Life.
	for turn = 0; turn < p.Turns; turn++ {

		// prepare request for server
		req := stdstruct.CalRequest{
			StartY: 0,
			EndY:   p.ImageHeight,
			StartX: 0,
			EndX:   p.ImageWidth,
			World:  world,
		}
		var res stdstruct.CalResponse

		err := client.Call("GameOfLife.CalculateNextTurn", req, &res)
		if err != nil {
			fmt.Println("Error calculating next turn:", err)
			return
		}

		//update the world
		world = res.World
		c.completedTurns = turn + 1

		c.events <- TurnComplete{CompletedTurns: c.completedTurns}

		select {
		// ticker.C is a channel that receives ticks every 2 seconds
		case <-ticker.C:
			c.events <- AliveCellsCount{c.completedTurns, len(calculateAliveCells(p, world))}
		case key := <-c.keyPresses:
			switch key {
			case 's':
				c.events <- StateChange{c.completedTurns, Executing}
				outputImage(c, p, world)
			case 'q':
				// Server still alive, controller down
				outputImage(c, p, world)
				c.events <- StateChange{turn, Quitting}
				close(c.events)

			//all components of the distributed system are shut down
			case 'k':
				// Both server and controller down
				outputImage(c, p, world)
				fmt.Println("Shutting down the system ")
				var shutdownReq, shutdownRes struct{}
				client.Call("GameOfLife.ShutDown", &shutdownReq, &shutdownRes)
				c.ioCommand <- ioCheckIdle
				<-c.ioIdle
				c.events <- FinalTurnComplete{CompletedTurns: c.completedTurns, Alive: calculateAliveCells(p, world)}
				c.events <- StateChange{CompletedTurns: c.completedTurns, NewState: Quitting}
				close(c.events)

			case 'p':
				c.events <- StateChange{turn, Paused}
				pause := true

				for pause {
					key := <-c.keyPresses
					switch key {
					case 'p':
						fmt.Println("Continuing")
						c.events <- StateChange{turn, Executing}
						pause = false
					case 's':
						outputImage(c, p, world)
					case 'q':
						outputImage(c, p, world)
						c.ioCommand <- ioCheckIdle
						<-c.ioIdle
						c.events <- FinalTurnComplete{CompletedTurns: c.completedTurns, Alive: calculateAliveCells(p, world)}
						c.events <- StateChange{turn, Quitting}
						close(c.events)
						return
					}
				}
			}
		default:
		}

	}

	outputImage(c, p, world)

	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.events <- FinalTurnComplete{CompletedTurns: c.completedTurns, Alive: calculateAliveCells(p, world)}
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{c.completedTurns, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
