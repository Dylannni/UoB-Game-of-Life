package gol

import (
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

type distributorChannels struct {
	events         chan<- Event
	keyPresses     <-chan rune
	completedTurns int
	shared         *shareState // Implements shared state between distributor and io goroutine
}

var mu sync.Mutex

// tempWorld store each workers result
func worker(startY, endY, startX, endX int, p Params, world [][]byte, c distributorChannels, tempWorld *[][][]byte, i int, wg *sync.WaitGroup) {
	defer wg.Done()
	worldPart := calculateNextState(startY, endY, startX, endX, p, world, c)
	mu.Lock()
	(*tempWorld)[i] = worldPart
	mu.Unlock()

	fmt.Printf("Worker %d finished\n", i)
}

// send the world into output
func outputImage(c distributorChannels, p Params, world [][]byte) {
	//c.ioCommand <- ioOutput
	c.shared.commandLock.Lock()
	c.shared.command = ioOutput
	fmt.Println("distributor: Sending ioOutput command")
	c.shared.commandCond.Signal()
	c.shared.commandLock.Unlock()

	c.shared.filenameLock.Lock()
	c.shared.filename = strings.Join([]string{strconv.Itoa(p.ImageHeight), strconv.Itoa(p.ImageWidth), strconv.Itoa(c.completedTurns)}, "x")
	//c.ioFilename <- filename

	fmt.Printf("outputImage: Filename set to %s\n", c.shared.filename)
	c.shared.filenameCond.Signal()
	c.shared.filenameLock.Unlock()

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			//c.ioOutput <- world[y][x]
			c.shared.outputLock.Lock()
			c.shared.output = world[y][x]
			fmt.Printf("outputImage: Output data set for cell (%d, %d) with value %d\n", y, x, world[y][x])
			c.shared.outputCond.Signal()
			c.shared.outputCond.Signal()
			for c.shared.output != 0 {
				c.shared.outputCond.Wait()
			}
			c.shared.outputLock.Unlock()
		}
	}

	c.shared.commandLock.Lock()
	c.shared.command = ioCheckIdle
	fmt.Println("distributor: Sending ioCheckIdle (exit) command")
	c.shared.commandCond.Signal()
	c.shared.commandLock.Unlock()

	c.shared.idleLock.Lock()
	for !c.shared.idle {
		c.shared.idleCond.Wait()
	}
	c.shared.idle = false
	c.shared.idleLock.Unlock()
	c.events <- ImageOutputComplete{c.completedTurns, c.shared.filename}

}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	// TODO: Create a 2D slice to store the world.
	world := initWorld(p.ImageHeight, p.ImageWidth)

	ticker := time.NewTicker(2 * time.Second)

	//c.ioCommand <- ioInput
	c.shared.commandLock.Lock()
	c.shared.command = ioInput
	fmt.Println("distributor: Sending ioInput command")
	c.shared.commandCond.Signal()
	c.shared.commandLock.Unlock()

	c.shared.filenameLock.Lock()

	c.shared.filename = strings.Join([]string{strconv.Itoa(p.ImageHeight), strconv.Itoa(p.ImageWidth)}, "x")
	c.shared.filenameCond.Signal()
	c.shared.filenameLock.Unlock()

	// add value to the input
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.shared.inputLock.Lock()

			for c.shared.input == 0 {
				c.shared.inputCond.Wait()
			}
			//val := <-c.ioInput
			world[y][x] = c.shared.input
			fmt.Printf("distributor: Input data received for cell (%d, %d): %d\n", y, x, c.shared.input)
			//world[y][x] = val
			c.shared.input = 0
			fmt.Printf("distributor: Signaling input reset for cell (%d, %d)\n", y, x)
			c.shared.inputCond.Signal()
			c.shared.inputLock.Unlock()

			fmt.Println("recieve alive cells")
			if world[y][x] == 255 {
				c.events <- CellFlipped{CompletedTurns: 0, Cell: util.Cell{X: x, Y: y}}
				fmt.Println("done")
			}
		}
	}

	turn := 0
	fmt.Println("set p = 0")
	c.events <- StateChange{turn, Executing}
	fmt.Println("distributor: Starting main simulation loop")

	// TODO: Execute all turns of the Game of Life.
	for turn = 0; turn < p.Turns; turn++ {
		fmt.Printf("distributor: Starting turn %d\n", turn)
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
		fmt.Printf("distributor: Turn %d complete\n", turn)

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
				// c.ioCommand <- ioCheckIdle
				// <-c.ioIdle
				c.shared.commandLock.Lock()
				c.shared.command = ioCheckIdle
				c.shared.commandCond.Signal()
				c.shared.commandLock.Unlock()

				c.shared.idleLock.Lock()
				for !c.shared.idle {
					c.shared.idleCond.Wait()
				}
				c.shared.idle = false
				c.shared.idleLock.Unlock()
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
						// c.ioCommand <- ioCheckIdle
						// <-c.ioIdle
						c.shared.commandLock.Lock()
						c.shared.command = ioCheckIdle
						c.shared.commandCond.Signal()
						c.shared.commandLock.Unlock()

						c.shared.idleLock.Lock()
						for !c.shared.idle {
							c.shared.idleCond.Wait()
						}
						c.shared.idle = false
						c.shared.idleLock.Unlock()
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
	// c.ioCommand <- ioCheckIdle
	// <-c.ioIdle
	c.shared.commandLock.Lock()
	c.shared.command = ioCheckIdle
	c.shared.commandCond.Signal()
	c.shared.commandLock.Unlock()

	c.shared.idleLock.Lock()
	for !c.shared.idle {
		c.shared.idleCond.Wait()
	}
	c.shared.idle = false
	c.shared.idleLock.Unlock()

	c.events <- StateChange{c.completedTurns, Quitting}

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	close(c.events)
}
