package gol

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"

	"uk.ac.bris.cs/gameoflife/util"
)

type shareState struct {
	commandLock sync.Mutex // control access to shared variables
	command     ioCommand
	commandCond *sync.Cond // condition variable -allows goroutine to wait for a specific condition (such as data being ready) to hold.

	idleLock sync.Mutex
	idle     bool
	idleCond *sync.Cond

	filenameLock sync.Mutex
	filename     string
	filenameCond *sync.Cond

	outputLock sync.Mutex
	output     uint8
	outputCond *sync.Cond

	inputLock sync.Mutex
	input     uint8
	inputCond *sync.Cond

	//outputBuffer [][]uint8
}

// ioState is the internal ioState of the io goroutine.
type ioState struct {
	params Params
	shared *shareState
}

func NewShareState() *shareState {
	ioState := &shareState{}
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

	// Request a filename from the distributor.
	//filename := <-io.channels.filename
	io.shared.filenameLock.Lock()
	defer io.shared.filenameLock.Unlock()
	for io.shared.filename == "" {
		fmt.Println("writePgmImage: Waiting for filename...")
		io.shared.filenameCond.Wait()
	}
	filename := io.shared.filename

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

	for y := 0; y < io.params.ImageHeight; y++ {
		for x := 0; x < io.params.ImageWidth; x++ {
			//val := <-io.channels.output
			//if val != 0 {
			//	fmt.Println(x, y)
			//}
			io.shared.outputLock.Lock()
			for io.shared.output == 0 {
				fmt.Println("writePgmImage: Waiting for output data...")
				io.shared.outputCond.Wait() // wait for output update
			}
			val := io.shared.output
			io.shared.output = 0
			io.shared.outputCond.Signal() //signal other goroutine
			fmt.Printf("writePgmImage: Received output data at (%d, %d): %d\n", x, y, val)
			io.shared.outputLock.Unlock()
			world[y][x] = val
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

	// Request a filename from the distributor.
	//filename := <-io.channels.filename
	io.shared.filenameLock.Lock()
	defer io.shared.filenameLock.Unlock()
	for io.shared.filename == "" {
		fmt.Println("readPgmImage: Waiting for filename...")
		io.shared.filenameCond.Wait()
	}
	filename := io.shared.filename

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
		//io.channels.input <- b
		io.shared.inputLock.Lock()
		io.shared.input = b
		io.shared.inputCond.Signal()
		fmt.Printf("readPgmImage: Sent input data: %d\n", b)
		for io.shared.input != 0 {
			io.shared.inputCond.Wait()
		}
		io.shared.inputLock.Unlock()
	}

	fmt.Println("File", filename, "input done!")
}

// startIo should be the entrypoint of the io goroutine.
func startIo(p Params, shared *shareState) {
	io := ioState{
		params: p,
		shared: shared,
	}

	for {
		io.shared.commandLock.Lock()
		for io.shared.command == 0 {
			fmt.Println("startIo: Waiting for command...")
			io.shared.commandCond.Wait()
		}
		command := io.shared.command
		fmt.Println("startIo: Received command:", command)
		io.shared.command = 0
		io.shared.commandLock.Unlock()
		// Block and wait for requests from the distributor
		switch command {
		case ioInput:
			io.readPgmImage()
			fmt.Println("done")
		case ioOutput:
			io.writePgmImage()
		case ioCheckIdle:
			//io.channels.idle <- true
			io.shared.idleLock.Lock()
			io.shared.idle = true
			fmt.Println("startIo: Idle state set to true")
			io.shared.idleCond.Signal()
			io.shared.idleLock.Unlock()
		}
	}
}
