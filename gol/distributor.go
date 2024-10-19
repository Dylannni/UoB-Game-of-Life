package gol

import (
	"strconv"
	"strings"
	"time"
)

type distributorChannels struct {
	events         chan<- Event
	ioCommand      chan<- ioCommand
	ioIdle         <-chan bool
	ioFilename     chan<- string
	ioOutput       chan<- uint8
	ioInput        <-chan uint8
	completedTurns int
}

func worker(startY, endY, startX, endX int, p Params, world [][]byte, c distributorChannels, tempWorld chan<- [][]byte) {
	worldPart := calculateNextState(startY, endY, startX, endX, p, world, c)
	tempWorld <- worldPart
}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	// TODO: Create a 2D slice to store the world.
	world := initWorld(p.ImageHeight, p.ImageWidth)

	ticker := time.NewTicker(2 * time.Second)

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

		if p.Threads == 1 {
			world = calculateNextState(0, p.ImageHeight, 0, p.ImageWidth, p, world, c)
		} else {
			tempWorld := make([]chan [][]byte, p.Threads)
			for i := range tempWorld {
				tempWorld[i] = make(chan [][]byte)
			}

			heightPerThread := p.ImageHeight / p.Threads

			for i := 0; i < p.Threads-1; i++ {
				go worker(i*heightPerThread, (i+1)*heightPerThread, 0, p.ImageWidth, p, world, c, tempWorld[i])
			}
			go worker((p.Threads-1)*heightPerThread, p.ImageHeight, 0, p.ImageWidth, p, world, c, tempWorld[p.Threads-1])

			mergeWorld := initWorld(0, 0)
			for i := 0; i < p.Threads; i++ {
				pieces := <-tempWorld[i]
				mergeWorld = append(mergeWorld, pieces...)
			}
			world = mergeWorld
		}

		c.completedTurns = turn + 1
		c.events <- TurnComplete{CompletedTurns: c.completedTurns}

		select {
		case <-ticker.C:
			c.events <- AliveCellsCount{c.completedTurns, len(calculateAliveCells(p, world))}
		default:
		}
	}

	// TODO: Report the final state using FinalTurnCompleteEvent.
	c.events <- FinalTurnComplete{CompletedTurns: p.Turns, Alive: calculateAliveCells(p, world)}
	// Make sure that the Io has finished any output before exiting.
	c.ioCommand <- ioCheckIdle
	<-c.ioIdle

	c.events <- StateChange{turn, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
