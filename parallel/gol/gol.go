package gol

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

// Run starts the processing of Game of Life. It should initialise channels and goroutines.
func Run(p Params, events chan<- Event, keyPresses <-chan rune) {
	completedTurns := 0
	shared := NewShareState()

	go startIo(p, shared)

	sdl := NewSDLState()

	distributorChannels := distributorChannels{
		events:         events,
		completedTurns: completedTurns,
		keyPresses:     keyPresses,
		shared:         shared,
		sdl:            sdl,
	}
	go func() {
		for {
			outsideEvent := sdl.GetEvent()

			events <- outsideEvent
			if _, ok := outsideEvent.(FinalTurnComplete); ok {
				close(events)
				return
			}
		}
	}()

	distributor(p, distributorChannels)
}
