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

	// internalKeyPress []rune
	// keyPressLock     sync.Mutex
	// keyPressCond     *sync.Cond
}

type distributorChannels struct {
	events         chan<- Event
	keyPresses     <-chan rune
	sdl            *sdlState
	completedTurns int
	shared         *shareIOState
}

func NewSDLState() *sdlState {
	sdlstate := &sdlState{
		internalEvents: make([]Event, 0),
		//internalKeyPress: make([]rune, 0),
	}
	sdlstate.eventsCond = sync.NewCond(&sdlstate.eventsLock)
	//sdlstate.keyPressCond = sync.NewCond(&sdlstate.keyPressLock)
	return sdlstate
}

func (s *sdlState) AddEvent(event Event) {
	s.eventsLock.Lock()
	s.internalEvents = append(s.internalEvents, event)
	s.eventsCond.Signal()
	s.eventsLock.Unlock()
}

func (s *sdlState) GetEvent() Event {
	s.eventsLock.Lock()
	for len(s.internalEvents) == 0 {
		s.eventsCond.Wait()
	}
	event := s.internalEvents[0]
	s.internalEvents = s.internalEvents[1:]
	s.eventsLock.Unlock()
	return event
}

// func (s *sdlState) SetKeyPress(key rune) {
// 	s.keyPressLock.Lock()
// 	s.internalKeyPress = append(s.internalKeyPress, key)
// 	s.keyPressCond.Signal()
// 	s.keyPressLock.Unlock()
// }

// func (s *sdlState) WaitForKeyPress() rune {
// 	s.keyPressLock.Lock()
// 	defer s.keyPressLock.Unlock()
// 	s.keyPressCond.Wait()
// 	return s.internalKeyPress
// }

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
	c.shared.commandLock.Lock()
	c.shared.command = ioOutput
	c.shared.commandReady = true
	c.shared.commandCond.Signal()
	c.shared.commandLock.Unlock()

	c.shared.filenameLock.Lock()
	c.shared.filename = strings.Join([]string{strconv.Itoa(p.ImageHeight), strconv.Itoa(p.ImageWidth), strconv.Itoa(c.completedTurns)}, "x")
	c.shared.filenameReady = true
	c.shared.filenameCond.Signal()
	c.shared.filenameLock.Unlock()

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.shared.outputLock.Lock()
			c.shared.output = world[y][x]
			c.shared.outputReady = true
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

	c.shared.commandLock.Lock()
	c.shared.command = ioInput
	c.shared.commandReady = true
	c.shared.commandCond.Signal()
	c.shared.commandLock.Unlock()

	c.shared.filenameLock.Lock()

	c.shared.filename = strings.Join([]string{strconv.Itoa(p.ImageHeight), strconv.Itoa(p.ImageWidth)}, "x")
	c.shared.filenameReady = true
	c.shared.filenameCond.Signal()
	c.shared.filenameLock.Unlock()

	for y := 0; y < p.ImageHeight; y++ {
		for x := 0; x < p.ImageWidth; x++ {
			c.shared.inputLock.Lock()

			for !c.shared.inputReady {
				c.shared.inputCond.Wait()
			}
			world[y][x] = c.shared.input
			c.shared.inputReady = false
			c.shared.inputCond.Signal()
			c.shared.inputLock.Unlock()

			if world[y][x] == 255 {
				c.sdl.AddEvent(CellFlipped{CompletedTurns: 0, Cell: util.Cell{X: x, Y: y}})

			}
		}
	}

	turn := 0
	c.sdl.AddEvent(StateChange{turn, Executing})

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

			wg.Wait()

			mergeWorld := initWorld(0, 0)
			for i := 0; i < p.Threads; i++ {
				mergeWorld = append(mergeWorld, tempWorld[i]...)
			}
			world = mergeWorld
		}

		c.sdl.AddEvent(TurnComplete{CompletedTurns: c.completedTurns})

		select {
		//ticker.C is a channel that receives ticks every 2 seconds
		case <-ticker.C:
			c.sdl.AddEvent(AliveCellsCount{c.completedTurns, len(calculateAliveCells(p, world))})
		case key := <-c.keyPresses:
			switch key {
			case 's':
				c.sdl.AddEvent(StateChange{c.completedTurns, Executing})
				outputImage(c, p, world)
			case 'q':
				outputImage(c, p, world)

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
				c.sdl.AddEvent(StateChange{turn, Paused})
				pause := true

				for pause {
					key := <-c.keyPresses
					switch key {
					case 'p':
						c.sdl.AddEvent(StateChange{turn, Executing})
						pause = false
					case 's':
						outputImage(c, p, world)
					case 'q':
						outputImage(c, p, world)
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

}
