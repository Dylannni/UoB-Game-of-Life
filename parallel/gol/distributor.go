package gol

import (
	//"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/util"
)

type sdlState struct {
	eventsLock     sync.Mutex
	internalEvents []Event
	eventsCond     *sync.Cond

	internalKeyPress rune
	keyPressLock     sync.Mutex
	keyPressCond     *sync.Cond
}

// distributorChannels struct holds shared state and channels used by the distributor.
type distributorChannels struct {
	events         chan<- Event
	keyPresses     <-chan rune
	sdl            *sdlState
	completedTurns int
	shared         *shareIOState // Implements shared state between distributor and io goroutine
}

func NewSDLState() *sdlState {
	sdlstate := &sdlState{
		internalEvents: make([]Event, 0),
	}
	sdlstate.eventsCond = sync.NewCond(&sdlstate.eventsLock)
	sdlstate.keyPressCond = sync.NewCond(&sdlstate.keyPressLock)
	return sdlstate
}

func (s *sdlState) AddEvent(event Event) {
	s.eventsLock.Lock()
	defer s.eventsLock.Unlock()
	s.internalEvents = append(s.internalEvents, event)
	s.eventsCond.Signal()
}

func (s *sdlState) GetEvent() Event {
	s.eventsLock.Lock()
	defer s.eventsLock.Unlock()
	for len(s.internalEvents) == 0 {
		s.eventsCond.Wait()
	}
	event := s.internalEvents[0]
	s.internalEvents = s.internalEvents[1:]
	return event
}

func (s *sdlState) SetKeyPress(key rune) {
	s.keyPressLock.Lock()
	s.internalKeyPress = key
	s.keyPressCond.Signal()
	s.keyPressLock.Unlock()
}

func (s *sdlState) WaitForKeyPress() rune {
	s.keyPressLock.Lock()
	defer s.keyPressLock.Unlock()
	s.keyPressCond.Wait()
	return s.internalKeyPress
}

// Mutex for synchronizing worker functions
var mu sync.Mutex

// tempWorld store each workers result
func worker(startY, endY, startX, endX int, p Params, world [][]byte, c distributorChannels, tempWorld *[][][]byte, i int, wg *sync.WaitGroup) {
	defer wg.Done()
	worldPart := calculateNextState(startY, endY, startX, endX, p, world, c)
	mu.Lock()
	(*tempWorld)[i] = worldPart
	mu.Unlock()

	//fmt.Printf("Worker %d finished\n", i)
}

// send the world into output
func outputImage(c distributorChannels, p Params, world [][]byte) {
	//c.ioCommand <- ioOutput
	c.shared.commandLock.Lock()
	c.shared.command = ioOutput
	c.shared.commandReady = true
	//fmt.Println("distributor: Sending ioOutput command")
	c.shared.commandCond.Signal()
	c.shared.commandLock.Unlock()

	c.shared.filenameLock.Lock()
	c.shared.filename = strings.Join([]string{strconv.Itoa(p.ImageHeight), strconv.Itoa(p.ImageWidth), strconv.Itoa(c.completedTurns)}, "x")
	//c.ioFilename <- filename
	c.shared.filenameReady = true
	//fmt.Printf("outputImage: Filename set to %s\n", c.shared.filename)
	c.shared.filenameCond.Signal()
	c.shared.filenameLock.Unlock()

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			//c.ioOutput <- world[y][x]
			c.shared.outputLock.Lock()
			c.shared.output = world[y][x]
			c.shared.outputReady = true
			//fmt.Printf("outputImage: Output data set for cell (%d, %d) with value %d\n", y, x, world[y][x])
			c.shared.outputCond.Signal()
			for c.shared.outputReady {
				c.shared.outputCond.Wait()
			}
			c.shared.outputLock.Unlock()
		}
	}

	c.shared.commandLock.Lock()
	c.shared.command = ioCheckIdle
	c.shared.commandReady = true
	//fmt.Println("distributor: Sending ioCheckIdle (exit) command")
	c.shared.commandCond.Signal()
	c.shared.commandLock.Unlock()

	c.shared.idleLock.Lock()
	for !c.shared.idle {
		c.shared.idleCond.Wait()
	}
	c.shared.idle = false
	c.shared.idleLock.Unlock()
	c.sdl.AddEvent(ImageOutputComplete{c.completedTurns, c.shared.filename})

}

// distributor divides the work between workers and interacts with other goroutines.
func distributor(p Params, c distributorChannels) {

	// TODO: Create a 2D slice to store the world.
	world := initWorld(p.ImageHeight, p.ImageWidth)

	ticker := time.NewTicker(2 * time.Second)

	//c.ioCommand <- ioInput
	c.shared.commandLock.Lock()
	c.shared.command = ioInput
	c.shared.commandReady = true
	//fmt.Println("distributor: Sending ioInput command")
	c.shared.commandCond.Signal()
	c.shared.commandLock.Unlock()

	c.shared.filenameLock.Lock()

	c.shared.filename = strings.Join([]string{strconv.Itoa(p.ImageHeight), strconv.Itoa(p.ImageWidth)}, "x")
	c.shared.filenameReady = true
	c.shared.filenameCond.Signal()
	c.shared.filenameLock.Unlock()

	// add value to the input
	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.shared.inputLock.Lock()

			for !c.shared.inputReady {
				//fmt.Println("distributor: Waiting for input data...")
				c.shared.inputCond.Wait()
			}
			//val := <-c.ioInput
			world[y][x] = c.shared.input
			//fmt.Printf("distributor: Input data received for cell (%d, %d): %d\n", y, x, c.shared.input)
			c.shared.inputReady = false
			//fmt.Printf("distributor: Signaling input reset for cell (%d, %d)\n", y, x)
			c.shared.inputCond.Signal()
			c.shared.inputLock.Unlock()

			//fmt.Println("recieve alive cells")
			if world[y][x] == 255 {
				c.sdl.AddEvent(CellFlipped{CompletedTurns: 0, Cell: util.Cell{X: x, Y: y}})

			}
		}
	}

	turn := 0
	c.sdl.AddEvent(StateChange{turn, Executing})
	//fmt.Println("distributor: Starting main simulation loop")

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
				//fmt.Printf("distributor: Starting worker %d\n", i)
				go worker(i*heightPerThread, (i+1)*heightPerThread, 0, p.ImageWidth, p, world, c, &tempWorld, i, &wg)
			}
			wg.Add(1)
			//fmt.Printf("distributor: Starting last worker\n")
			go worker((p.Threads-1)*heightPerThread, p.ImageHeight, 0, p.ImageWidth, p, world, c, &tempWorld, p.Threads-1, &wg)

			//wait all the workers to finish
			wg.Wait()
			//fmt.Println("distributor: All workers finished")

			mergeWorld := initWorld(0, 0)
			for i := 0; i < p.Threads; i++ {
				mergeWorld = append(mergeWorld, tempWorld[i]...)
			}
			world = mergeWorld
		}

		c.sdl.AddEvent(TurnComplete{CompletedTurns: c.completedTurns})
		//fmt.Printf("distributor: Turn %d complete\n", turn)

		select {
		//ticker.C is a channel that receives ticks every 2 seconds
		case <-ticker.C:
			//fmt.Println("distributor: Tick received, calculating alive cells")
			c.sdl.AddEvent(AliveCellsCount{c.completedTurns, len(calculateAliveCells(p, world))})
		case key := <-c.keyPresses:
			//fmt.Printf("distributor: Key press detected: %c\n", key)
			switch key {
			case 's':
				//fmt.Println("distributor: Saving image state")
				c.sdl.AddEvent(StateChange{c.completedTurns, Executing})
				outputImage(c, p, world)
			case 'q':
				//fmt.Println("distributor: Quit command received, preparing to exit")
				outputImage(c, p, world)
				// c.ioCommand <- ioCheckIdle
				// <-c.ioIdle
				c.shared.commandLock.Lock()
				c.shared.command = ioCheckIdle
				c.shared.commandReady = true
				c.shared.commandCond.Signal()
				c.shared.commandLock.Unlock()

				c.shared.idleLock.Lock()
				for !c.shared.idle {
					c.shared.idleCond.Wait()
				}
				c.shared.idle = false
				c.shared.idleLock.Unlock()
				c.sdl.AddEvent(StateChange{turn, Quitting})
				c.sdl.AddEvent(FinalTurnComplete{CompletedTurns: c.completedTurns, Alive: calculateAliveCells(p, world)})
				return
			case 'p':
				//fmt.Println("distributor: Pausing simulation")
				c.sdl.AddEvent(StateChange{turn, Paused})
				//mu.Lock()
				pause := true
				//mu.Unlock()

				for pause {
					key := <-c.keyPresses
					//fmt.Printf("distributor: Key press during pause: %c\n", key)
					switch key {
					case 'p':
						//fmt.Println("distributor: Resuming simulation from pause")
						c.sdl.AddEvent(StateChange{turn, Executing})
						//mu.Lock()
						pause = false
						//mu.Unlock()
					case 's':
						//fmt.Println("distributor: Saving image state during pause")
						outputImage(c, p, world)
					case 'q':
						//fmt.Println("distributor: Quit command received during pause, preparing to exit")
						outputImage(c, p, world)
						// c.ioCommand <- ioCheckIdle
						// <-c.ioIdle
						c.shared.commandLock.Lock()
						c.shared.command = ioCheckIdle
						c.shared.commandReady = true
						c.shared.commandCond.Signal()
						c.shared.commandLock.Unlock()

						c.shared.idleLock.Lock()
						for !c.shared.idle {
							c.shared.idleCond.Wait()
						}
						c.shared.idle = false
						c.shared.idleLock.Unlock()
						c.sdl.AddEvent(StateChange{turn, Quitting})
						c.sdl.AddEvent(FinalTurnComplete{CompletedTurns: c.completedTurns, Alive: calculateAliveCells(p, world)})
						return
					}
				}
			}
		default:
		}

	}

	outputImage(c, p, world)
	//fmt.Println("distributor: All turns completed, saving final image")

	// TODO: Report the final state using FinalTurnCompleteEvent.
	// Make sure that the Io has finished any output before exiting.
	// c.ioCommand <- ioCheckIdle
	// <-c.ioIdle
	c.shared.commandLock.Lock()
	c.shared.command = ioCheckIdle
	c.shared.commandReady = true
	c.shared.commandCond.Signal()
	c.shared.commandLock.Unlock()

	c.shared.idleLock.Lock()
	for !c.shared.idle {
		c.shared.idleCond.Wait()
	}
	c.shared.idle = false
	c.shared.idleLock.Unlock()

	c.sdl.AddEvent(StateChange{c.completedTurns, Quitting})
	c.sdl.AddEvent(FinalTurnComplete{CompletedTurns: c.completedTurns, Alive: calculateAliveCells(p, world)})

	// Close the channel to stop the SDL goroutine gracefully. Removing may cause deadlock.
	//fmt.Println("distributor: Exiting simulation and closing events channel")
	//close(c.events)
}
