package gol

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"uk.ac.bris.cs/gameoflife/util"
)

type shareIOState struct {
	commandLock  sync.Mutex // control access to shared variables
	command      ioCommand  // stores the current command from the distributor
	commandCond  *sync.Cond // condition variable -allows goroutine to wait for a specific condition (such as data being ready) to hold.
	commandReady bool       // flag indicating if a command is ready

	idleLock sync.Mutex
	idle     bool
	idleCond *sync.Cond

	filenameLock  sync.Mutex
	filename      string
	filenameCond  *sync.Cond
	filenameReady bool

	outputLock  sync.Mutex
	output      uint8
	outputCond  *sync.Cond
	outputReady bool

	inputLock  sync.Mutex
	input      uint8
	inputCond  *sync.Cond
	inputReady bool
}

// ioState is the internal ioState of the io goroutine.
type ioState struct {
	params Params
	shared *shareIOState
}

// initializes and returns a new instance of shareState with its condition variables.
func NewShareState() *shareIOState {
	ioState := &shareIOState{}
	ioState.commandCond = sync.NewCond(&ioState.commandLock)
	ioState.idleCond = sync.NewCond(&ioState.idleLock)
	ioState.filenameCond = sync.NewCond(&ioState.filenameLock)
	ioState.outputCond = sync.NewCond(&ioState.outputLock)
	ioState.inputCond = sync.NewCond(&ioState.inputLock)
	return ioState

}

// ioCommand allows requesting behaviour from the io (pgm) goroutine.
type ioCommand uint8

// This is a way of creating enums in Go.
// It will evaluate to:
//
//	ioOutput 	= 0
//	ioInput 	= 1
//	ioCheckIdle = 2
const (
	ioOutput ioCommand = iota
	ioInput
	ioCheckIdle
)

// writePgmImage receives an array of bytes and writes it to a pgm file.
func (io *ioState) writePgmImage() {
	_ = os.Mkdir("out", os.ModePerm)

	io.shared.filenameLock.Lock()
	for !io.shared.filenameReady {
		io.shared.filenameCond.Wait()
	}
	filename := io.shared.filename
	io.shared.filenameReady = false
	io.shared.filenameCond.Signal()
	io.shared.filenameLock.Unlock()

	file, ioError := os.Create("out/" + filename + ".pgm")
	util.Check(ioError)
	defer file.Close()

	_, _ = file.WriteString("P5\n")
	//_, _ = file.WriteString("# PGM file writer by pnmmodules (https://github.com/owainkenwayucl/pnmmodules).\n")
	_, _ = file.WriteString(strconv.Itoa(io.params.ImageWidth))
	_, _ = file.WriteString(" ")
	_, _ = file.WriteString(strconv.Itoa(io.params.ImageHeight))
	_, _ = file.WriteString("\n")
	_, _ = file.WriteString(strconv.Itoa(255))
	_, _ = file.WriteString("\n")

	world := make([][]byte, io.params.ImageHeight)
	for i := range world {
		world[i] = make([]byte, io.params.ImageWidth)
	}

	// Write pixel data for each position in the world.
	for y := 0; y < io.params.ImageHeight; y++ {
		for x := 0; x < io.params.ImageWidth; x++ {
			io.shared.outputLock.Lock()
			for !io.shared.outputReady {
				io.shared.outputCond.Wait() // wait for output update
			}
			world[y][x] = io.shared.output
			io.shared.outputReady = false
			io.shared.outputCond.Signal() // tell other goroutine is finished
			io.shared.outputLock.Unlock()
		}
	}

	for y := 0; y < io.params.ImageHeight; y++ {
		for x := 0; x < io.params.ImageWidth; x++ {
			_, ioError = file.Write([]byte{world[y][x]})
			util.Check(ioError)
		}
	}

	ioError = file.Sync()
	util.Check(ioError)

	fmt.Println("File", filename, "output done!")
}

// readPgmImage opens a pgm file and sends its data as an array of bytes.
func (io *ioState) readPgmImage() {

	io.shared.filenameLock.Lock()
	for !io.shared.filenameReady {
		io.shared.filenameCond.Wait()
	}
	filename := io.shared.filename
	io.shared.filenameReady = false
	io.shared.filenameLock.Unlock()

	data, ioError := os.ReadFile("images/" + filename + ".pgm")
	util.Check(ioError)

	fields := strings.Fields(string(data))

	if fields[0] != "P5" {
		panic("Not a pgm file")
	}

	width, _ := strconv.Atoi(fields[1])
	if width != io.params.ImageWidth {
		panic("Incorrect width")
	}

	height, _ := strconv.Atoi(fields[2])
	if height != io.params.ImageHeight {
		panic("Incorrect height")
	}

	maxval, _ := strconv.Atoi(fields[3])
	if maxval != 255 {
		panic("Incorrect maxval/bit depth")
	}

	image := []byte(fields[4])

	for _, b := range image {
		io.shared.inputLock.Lock()
		io.shared.input = b
		io.shared.inputReady = true // set that data is ready
		io.shared.inputCond.Signal()
		for io.shared.inputReady {
			io.shared.inputCond.Wait() // Wait for the consumer to reset inputReady to false
		}
		io.shared.inputLock.Unlock()
	}

	fmt.Println("File", filename, "input done!")
}

// startIo should be the entrypoint of the io goroutine.
func startIo(p Params, shared *shareIOState) {
	io := ioState{
		params: p,
		shared: shared,
	}

	for {
		io.shared.commandLock.Lock()
		for !io.shared.commandReady {
			io.shared.commandCond.Wait()
		}
		command := io.shared.command
		io.shared.commandReady = false
		io.shared.commandLock.Unlock()
		switch command {
		case ioInput:
			io.readPgmImage()

		case ioOutput:
			io.writePgmImage()
		case ioCheckIdle:
			io.shared.idleLock.Lock()
			io.shared.idle = true
			io.shared.idleCond.Signal()
			io.shared.idleLock.Unlock()
		}
	}
}
