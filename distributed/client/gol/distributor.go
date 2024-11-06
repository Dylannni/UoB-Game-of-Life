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

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {
	// brokerAddr := flag.String("broker", "127.0.0.1:8030", "Address of broker instance")
	// flag.Parse()

	// client, err := rpc.Dial("tcp", *brokerAddr)
	client, err := rpc.Dial("tcp", "127.0.0.1:8030")

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
		gameReq := stdstruct.GameRequest{World: world, Threads: p.Threads}
		var gameRes stdstruct.GameResponse

		err := client.Call("Broker.RunGol", gameReq, &gameRes)
		if err != nil {
			fmt.Println("Error starting game:", err)
			return
		}

		world = gameRes.World
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
				return
			//all components of the distributed system are shut down
			case 'k':
				// Both server and controller down
				outputImage(c, p, world)
				fmt.Println("Shutting down the system ")
				var shutdownReq, shutdownRes struct{}
				client.Call("Broker.ShutDown", &shutdownReq, &shutdownRes)
				c.ioCommand <- ioCheckIdle
				<-c.ioIdle
				c.events <- FinalTurnComplete{CompletedTurns: c.completedTurns, Alive: calculateAliveCells(p, world)}
				c.events <- StateChange{CompletedTurns: c.completedTurns, NewState: Quitting}
				close(c.events)
				return
			case 'p':
				c.events <- StateChange{turn, Paused}
				pause := true

				for pause {
					key := <-c.keyPresses
					switch key {
					case 'p':
						fmt.Println("Continuing")
						c.events <- StateChange{turn, Executing}
						ticker.Reset(2 * time.Second)
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
