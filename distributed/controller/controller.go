package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"

	"uk.ac.bris.cs/gameoflife/client/util"
	"uk.ac.bris.cs/gameoflife/stdstruct"
)

type GameOfLife struct {
	world          [][]byte
	height         int
	firstLineSent  chan bool // 检测是否已经发送上下光环的通道
	lastLineSent   chan bool
	previousServer *rpc.Client // 自己的上下光环服务器rpc，这里保存的是rpc客户端的pointer，
	nextServer     *rpc.Client // 这样就不用每次获取光环时都需要连接服务器了
}

func (s *GameOfLife) CheckWorking(_ struct{}, response *bool) error {
	*response = true
	return nil
}

func (s *GameOfLife) Init(req stdstruct.InitRequest, _ *stdstruct.InitResponse) (err error) {
	s.previousServer, err = rpc.Dial("tcp", req.PreviousServer)
	if err != nil {
		return fmt.Errorf("failed to connect to previous server: %v", err)
	}

	s.nextServer, err = rpc.Dial("tcp", req.NextServer)
	if err != nil {
		return fmt.Errorf("failed to connect to next server: %v", err)
	}

	s.world = req.World
	s.firstLineSent = make(chan bool)
	s.lastLineSent = make(chan bool)
	s.height = req.Height
	return nil
}

func attendHaloArea(height int, world [][]byte, topHalo, bottomHalo []byte) [][]byte {
	newWorld := make([][]byte, 0, height+2)
	newWorld = append(newWorld, topHalo)
	newWorld = append(newWorld, world...)
	newWorld = append(newWorld, bottomHalo)
	return newWorld
}

func (s *GameOfLife) GetFirstLine(_ stdstruct.HaloRequest, res *stdstruct.HaloResponse) (err error) {
	haloLine := make([]byte, len(s.world[0])) // 创建一个长度和世界第一行相同的列表（其实这里直接用s.width会更好）
	for i, val := range s.world[0] {
		haloLine[i] = val // 将世界第一行每个值复制进新的数组（这样即使世界被修改光环也肯定不会变）
	}
	res.HaloLine = haloLine
	s.firstLineSent <- true // 在交换前向通道传递值，这样保证所有服务器都完成光环交换后再继续运行下回合
	return
}

func (s *GameOfLife) GetLastLine(_ stdstruct.HaloRequest, res *stdstruct.HaloResponse) (err error) {
	height := len(s.world)
	haloLine := make([]byte, len(s.world[height-1]))
	for i, val := range s.world[height-1] {
		haloLine[i] = val
	}
	res.HaloLine = haloLine
	s.lastLineSent <- true
	return
}

func getHalo(server *rpc.Client, isFirstLine bool, out chan []byte) {
	res := stdstruct.HaloResponse{}
	var err error
	if isFirstLine {
		err = server.Call("GameOfLife.GetFirstLine", stdstruct.HaloRequest{}, &res)
		if err != nil {
			fmt.Println("Error getting first line:", err)
		}
	} else {
		err = server.Call("GameOfLife.GetLastLine", stdstruct.HaloRequest{}, &res)
		if err != nil {
			fmt.Println("Error getting last line:", err)
		}
	}
	out <- res.HaloLine
}

// countLiveNeighbors calculates the number of live neighbors for a given cell.
// Parameters:
//   - world: A 2D byte array representing the state of the world, where 255 indicates a live cell, and 0 indicates a dead cell.
//   - row(globalY), col(globalX): The row and column of the current cell to calculate neighbors for.
//   - rows(p.ImageHeight), cols(p.ImageWidth): The total number of rows and columns in the world, used for boundary handling.
//
// Returns:
//   - The number of live neighboring cells.
func countLiveNeighbors(world [][]byte, row, col, rows, cols int) int {
	neighbors := [8][2]int{
		{-1, -1}, {-1, 0}, {-1, 1}, // Top-left, Top, Top-right
		{0, -1}, {0, 1}, // Left, Right
		{1, -1}, {1, 0}, {1, 1}, // Bottom-left, Bottom, Bottom-right
	}

	liveNeighbors := 0
	for _, n := range neighbors {
		// Ensures the world wraps around at the edges (i.e. torus-like world)
		newRow := (row + n[0] + rows) % rows
		newCol := (col + n[1] + cols) % cols

		// Example: At a 5x5 world, if the current cell is at (0,0) and the neighbor is {-1, -1} (Top-left),
		// the newRow and newCol would be calculated as:
		// newRow = (0 + (-1) + 5) % 5 = 4  (wraps around to the bottom row)
		// newCol = (0 + (-1) + 5) % 5 = 4  (wraps around to the rightmost column)
		// So, the Top-left neighbor of (0, 0) would be (4, 4), wrapping around from the bottom-right.

		if world[newRow][newCol] == 255 {
			liveNeighbors++
		}
	}
	return liveNeighbors
}

func calculateNextTurn(localStartY, height, width, globalstartY int, extendedWorld [][]byte) []util.Cell {
	var flippedCells []util.Cell
	// Iterate over each cell in the world
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			globalY := localStartY + y + 1
			globalX := x
			// Count the live neighbors
			liveNeighbors := countLiveNeighbors(extendedWorld, globalY, globalX, len(extendedWorld), len(extendedWorld[0]))
			// Apply the Game of Life rules
			if extendedWorld[globalY][globalX] == 255 && (liveNeighbors < 2 || liveNeighbors > 3) {
				flippedCells = append(flippedCells, util.Cell{X: x, Y: globalstartY + localStartY + y})
			} else if extendedWorld[globalY][globalX] == 0 && liveNeighbors == 3 {
				flippedCells = append(flippedCells, util.Cell{X: x, Y: globalstartY + localStartY + y})
			}
		}
	}
	return flippedCells
}

func worker(startY, height, width, globalstartY int, extendedWorld [][]byte, flippedCellsCh chan<- []util.Cell) {
	cellFlippeds := calculateNextTurn(startY, height, width, globalstartY, extendedWorld)
	flippedCellsCh <- cellFlippeds
}

func (s *GameOfLife) NextTurn(req *stdstruct.SliceRequest, res *stdstruct.SliceResponse) (err error) {

	preOut := make(chan []byte)
	nextOut := make(chan []byte)

	go getHalo(s.previousServer, false, preOut)
	go getHalo(s.nextServer, true, nextOut)

	// Wait for neigbour node to send the getHalo() request
	<-s.firstLineSent
	<-s.lastLineSent

	topHalo := <-preOut
	bottomHalo := <-nextOut

	height := req.EndY - req.StartY
	width := req.EndX - req.StartX

	// world slice with two extra row (one at the top and one at the bottom)
	extendedWorld := attendHaloArea(height, s.world, topHalo, bottomHalo)

	globalstartY := req.StartY
	heightPerThread := height / req.Threads

	var flippedCellsCh []chan []util.Cell // list of flipped cells channel that yet to merge
	for i := 0; i < req.Threads; i++ {
		flippedCellCh := make(chan []util.Cell)
		flippedCellsCh = append(flippedCellsCh, flippedCellCh)

		var workerHeight int
		startY := i * heightPerThread
		if i == req.Threads-1 {
			workerHeight = height - startY
		} else {
			workerHeight = heightPerThread
		}
		go worker(startY, workerHeight, width, globalstartY, extendedWorld, flippedCellCh)
	}

	var cellFlippeds []util.Cell
	for i := 0; i < req.Threads; i++ {
		cellFlippeds = append(cellFlippeds, <-flippedCellsCh[i]...)
	}
	res.FlippedCells = cellFlippeds
	return nil
}

// shutting down the server when k is pressed
func (s *GameOfLife) ShutDown(_ *stdstruct.ShutRequest, _ *stdstruct.ShutResponse) (err error) {
	fmt.Println("Shutting down the server")
	os.Exit(0)
	return nil
}

func main() {
	// Usage: go run controller.go -port XXXX

	// Default port 8080
	pAddr := flag.String("port", "8080", "Port to listen on")
	flag.Parse()
	rpc.Register(&GameOfLife{})
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	fmt.Println("Controller started, listening on port", *pAddr)
	rpc.Accept(listener)
}
