package main

import (
	"flag"
	"fmt"
	"os"
	"sync"
	"net"
	"net/rpc"

	"bytes"


	"uk.ac.bris.cs/gameoflife/stdstruct"
)

type GameOfLife struct{
	world 			[][]byte
	height 			int
	// width			int
	firstLineSent  	chan bool // 检测是否已经发送上下光环的通道
	lastLineSent   	chan bool
	previousServer 	*rpc.Client // 自己的上下光环服务器rpc，这里保存的是rpc客户端的pointer，
	nextServer     	*rpc.Client // 这样就不用每次获取光环时都需要连接服务器了
	mu 				sync.Mutex
}

func (s *GameOfLife) Init(req stdstruct.InitRequest, _ *stdstruct.InitResponse) (err error) {

	s.previousServer, err = rpc.Dial("tcp", req.PreviousServer)
	if err != nil {
		return fmt.Errorf("failed to connect to previous server: %v", err)
	}
	fmt.Println("Connect to previous halo server ", req.PreviousServer)

	s.nextServer, err = rpc.Dial("tcp", req.NextServer)
	if err != nil {
		return fmt.Errorf("failed to connect to next server: %v", err)
	}
	fmt.Println("Connect to next halo server ", req.NextServer)


	s.world = req.World
	s.firstLineSent = make(chan bool)
	s.lastLineSent = make(chan bool)
	s.height = req.Height
	fmt.Println("INITED GameOfLife")
	return nil
}

func attendHaloArea(height int, world [][]byte, topHalo, bottomHalo []byte) [][]byte {
	newWorld := make([][]byte, 0, height+2)
	newWorld = append(newWorld, topHalo)
	newWorld = append(newWorld, world...)
	newWorld = append(newWorld, bottomHalo)
	return newWorld
}

// GetFirstLine 允许其他服务器调用，调用时会返回自己世界第一行的数据，完成后向通道传递信息
func (s *GameOfLife) GetFirstLine(_ stdstruct.HaloRequest, res *stdstruct.HaloResponse) (err error) {
	// 这里不用互斥锁的原因是服务器在交换光环的过程中是阻塞的，不会修改世界的数据
	haloLine := make([]byte, len(s.world[0])) // 创建一个长度和世界第一行相同的列表（其实这里直接用s.width会更好）
	for i, val := range s.world[0] {
		haloLine[i] = val // 将世界第一行每个值复制进新的数组（这样即使世界被修改光环也肯定不会变）
	}
	res.HaloLine = haloLine
	s.firstLineSent <- true // 在交换前向通道传递值，这样保证所有服务器都完成光环交换后再继续运行下回合
	return
}

// GetLastLine 返回自己世界最后一行的数据，和 GetFirstLine 逻辑相同
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

// getHalo 是获取光环的函数，输入服务器地址和要获取的光环类型，然后调用指定服务器的方法，向通道传输返回值
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

// func (s *GameOfLife) Init(req *stdstruct.InitRequest, _ *stdstruct.InitResponse) {
// 	// init the AWS node, should move these to a seprate function
// 	s.world = req.World
// 	s.firstLineSent = make(chan bool)
// 	s.lastLineSent = make(chan bool)
// 	s.height = req.EndY - req.StartY
// 	s.width = req.EndX - req.StartX
// 	fmt.Println("Connect to halo server ", req.PreviousServer.Address+":"+req.PreviousServer.Port)

// 	// s.previousServer, _= rpc.Dial("tcp", req.PreviousServer.Address+":"+req.PreviousServer.Port)
// 	// fmt.Println("Connect to previous halo server ", req.PreviousServer.Address+":"+req.PreviousServer.Port)
// 	// s.nextServer, _ = rpc.Dial("tcp", req.NextServer.Address+":"+req.NextServer.Port)
// 	// fmt.Println("Connect to next halo server ", req.NextServer.Address+":"+req.NextServer.Port)
// }

func (s *GameOfLife) CalculateNextTurn(req *stdstruct.SliceRequest, res *stdstruct.SliceResponse) (err error) {

	// previousServer, _= rpc.Dial("tcp", req.PreviousServer.Address+":"+req.PreviousServer.Port)
	// fmt.Println("Connect to previous halo server ", req.PreviousServer.Address+":"+req.PreviousServer.Port)
	// nextServer, _ = rpc.Dial("tcp", req.NextServer.Address+":"+req.NextServer.Port)
	// fmt.Println("Connect to next halo server ", req.NextServer.Address+":"+req.NextServer.Port)

	// var previousServer *rpc.Client
	// var nextServer *rpc.Client


	// // previousServer, _ = rpc.Dial("tcp", req.PreviousServer)
	// previousServer, err = rpc.Dial("tcp", req.PreviousServer)
	// if err != nil {
	// 	return fmt.Errorf("failed to connect to previous server: %v", err)
	// }
	// fmt.Println("Connect to previous halo server ", req.PreviousServer)
	// // nextServer, _ = rpc.Dial("tcp", req.NextServer)
	// // fmt.Println("Connect to next halo server ", req.NextServer)

	// nextServer, err = rpc.Dial("tcp", req.NextServer)
	// if err != nil {
	// 	return fmt.Errorf("failed to connect to next server: %v", err)
	// }
	// fmt.Println("Connect to next halo server ", req.PreviousServer)

	// Two Channels used to recive Halo Area from getHalo()

	extWorld := req.ExtendedSlice

	preOut := make(chan []byte)
	nextOut := make(chan []byte)

	go getHalo(s.previousServer, false, preOut)
	go getHalo(s.nextServer, true, nextOut)

	// Wait for neigbour node to send the getHalo() request
	<- s.firstLineSent
	<- s.lastLineSent

	topHalo := <-preOut
	bottomHalo := <-nextOut


	// 比较 topHalo 和 extWorld 的第一行
	// fmt.Println("Top Halo Line!!!!")
	// fmt.Println(bytes.Equal(topHalo, extWorld[0]))

	// 比较 bottomHalo 和 extWorld 的最后一行
	// if len(extWorld) > 0 {
	// 	lastIndex := len(extWorld) - 1
	// 	fmt.Println("Bottom Halo Line!!!!")
	// 	fmt.Println(bytes.Equal(bottomHalo, extWorld[lastIndex]))
	// } else {
	// 	fmt.Println("Error: extWorld is empty.")
	// }

	height := req.EndY - req.StartY
	width := req.EndX - req.StartX

	// world slice with two extra row (one at the top and one at the bottom)
	currWorld := attendHaloArea(height, req.Slice, topHalo, bottomHalo)
	// currWorld := req.ExtendedSlice

	fmt.Println("Halo World!!!!")

	fmt.Println(bytes.Equal(currWorld, extWorld))

	// world slice without halo area, will return to broker after calculation 
	nextWorld := req.Slice

	// Iterate over each cell in the world
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {

			globalY := y + 1
			globalX := x
			// Count the live neighbors
			liveNeighbors := countLiveNeighbors(currWorld, globalY, globalX, len(currWorld), len(currWorld[0]))
			// Apply the Game of Life rules
			if currWorld[globalY][globalX] == 255 {
				// Cell is alive
				if liveNeighbors < 2 || liveNeighbors > 3 {
					nextWorld[y][x] = 0 // Cell dies
				} else {
					nextWorld[y][x] = 255 // Cell stays alive
				}
			} else {
				// Cell is dead
				if liveNeighbors == 3 {
					nextWorld[y][x] = 255 // Cell becomes alive
				} else {
					nextWorld[y][x] = 0 // Cell stays dead
				}
			}
		}
	}
	res.Slice = nextWorld
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
