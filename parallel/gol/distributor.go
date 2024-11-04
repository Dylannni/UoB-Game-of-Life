package gol

import (
	"strconv"
	"strings"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
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

var mu sync.Mutex

// tempWorld store each workers result
func worker(startY, endY, startX, endX int, p Params, world [][]byte, c distributorChannels, tempWorld *[][][]byte, i int, wg *sync.WaitGroup) {
	defer wg.Done()
	worldPart := calculateNextState(startY, endY, startX, endX, p, world, c)
	mu.Lock()
	(*tempWorld)[i] = worldPart
	mu.Unlock()

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

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

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
		c.completedTurns = turn + 1

		if p.Threads == 1 {
			world = calculateNextState(0, p.ImageHeight, 0, p.ImageWidth, p, world, c)
		} else {
			tempWorld := make([][][]byte, p.Threads)
			heightPerThread := p.ImageHeight / p.Threads
			for i := range tempWorld {
				tempWorld[i] = make([][]byte, heightPerThread)
				for j := range tempWorld[i] {
					tempWorld[i][j] = make([]byte, p.ImageWidth)
				}
			}
			var wg sync.WaitGroup
			for i := 0; i < p.Threads-1; i++ {
				wg.Add(1)
				go worker(i*heightPerThread, (i+1)*heightPerThread, 0, p.ImageWidth, p, world, c, &tempWorld, i, &wg)
			}
			wg.Add(1)
			go worker((p.Threads-1)*heightPerThread, p.ImageHeight, 0, p.ImageWidth, p, world, c, &tempWorld, p.Threads-1, &wg)

			//wait all the workers to finish
			wg.Wait()

			mergeWorld := initWorld(0, 0)
			for i := 0; i < p.Threads; i++ {
				mergeWorld = append(mergeWorld, tempWorld[i]...)
			}
			world = mergeWorld
		}

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
				outputImage(c, p, world)
				c.ioCommand <- ioCheckIdle
				<-c.ioIdle
				c.events <- FinalTurnComplete{CompletedTurns: c.completedTurns, Alive: calculateAliveCells(p, world)}
				c.events <- StateChange{turn, Quitting}
				close(c.events)
				return
			case 'p':
				c.events <- StateChange{turn, Paused}
				//mu.Lock()
				pause := true
				//mu.Unlock()

				for pause {
					key := <-c.keyPresses
					switch key {
					case 'p':
						c.events <- StateChange{turn, Executing}
						mu.Lock()
						pause = false
						mu.Unlock()
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
