package server

//import "uk.ac.bris.cs/gameoflife/stdstruct"

//import "uk.ac.bris.cs/gameoflife/stdstruct"

// Params provides the details of how to run the Game of Life and which image to load.
type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

// ioCommand allows requesting behaviour from the io (pgm) goroutine.
// type ioCommand uint8

// This is a way of creating enums in Go.
// It will evaluate to:
//
//	ioOutput 	= 0
//	ioInput 	= 1
//	ioCheckIdle = 2
// const (
// 	ioOutput ioCommand = iota
// 	ioInput
// 	ioCheckIdle
// )

// type distributorChannels struct {
// 	events         chan<- Event
// 	ioCommand      chan<- ioCommand
// 	ioIdle         <-chan bool
// 	ioFilename     chan<- string
// 	ioOutput       chan<- uint8
// 	ioInput        <-chan uint8
// 	completedTurns int
// 	keyPresses     <-chan rune
// }

// type caculated struct {
// 	caculatedWorld  [][]byte
// 	AliveCellsCount []stdstruct.Cell
// }
