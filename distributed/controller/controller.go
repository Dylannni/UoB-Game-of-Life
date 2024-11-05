package main

import (
	"flag"
	"fmt"
	"os"
	"runtime"
	"sync"

	"net"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/stdstruct"
)

type GameOfLife struct{}

// initialise world
func InitWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
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

//worker: Responsible for computing a specified portion of the grid
func worker(startY, endY int, currWorld, nextWorld [][]byte, width int, resultCh chan<- []stdstruct.Cell, mu *sync.Mutex) {
	//使用本地的 aliveCells 列表存储局部计算的活细胞
	localAliveCells := []stdstruct.Cell{}
	for y := startY; y < endY; y++ {
		for x := 0; x < width; x++ {
			globalY := y
			globalX := x
			if globalY >= len(currWorld) || globalX >= width {
				continue // 跳过无效索引，避免越界
			}
			liveNeighbors := countLiveNeighbors(currWorld, globalY, globalX, len(currWorld), width)

			if currWorld[globalY][globalX] == 255 {
				if liveNeighbors < 2 || liveNeighbors > 3 {
					nextWorld[y][x] = 0
				} else {
					nextWorld[y][x] = 255
					localAliveCells = append(localAliveCells, stdstruct.Cell{X: globalX, Y: globalY})
				}
			} else {
				if liveNeighbors == 3 {
					nextWorld[y][x] = 255
					localAliveCells = append(localAliveCells, stdstruct.Cell{X: globalX, Y: globalY})
				} else {
					nextWorld[y][x] = 0
				}
			}
		}
	}
	// 使用互斥锁来安全地将本地的 aliveCells 合并到全局的 aliveCells 中
	mu.Lock()
	resultCh <- localAliveCells
	mu.Unlock()
}

func (s *GameOfLife) CalculateNextTurn(req *stdstruct.CalRequest, res *stdstruct.CalResponse) (err error) {

	currWorld := InitWorld(req.EndY, req.EndX)
	for y := 0; y < req.EndY; y++ {
		for x := 0; x < req.EndX; x++ {
			currWorld[y][x] = req.World[y][x]
		}
	}

	height := req.EndY - req.StartY
	width := req.EndX - req.StartX
	nextWorld := InitWorld(height, width)

	//Gets the number of available CPU cores
	numWorkers := runtime.NumCPU()
	heightPerWorker := height / numWorkers

	var wg sync.WaitGroup
	var mu sync.Mutex
	resultCh := make(chan []stdstruct.Cell, numWorkers)
	wg.Add(numWorkers)

	for i := 0; i < numWorkers; i++ {
		startY := i * heightPerWorker
		endY := startY + heightPerWorker
		if i == numWorkers-1 {
			endY = height //确保最后一个worker 覆盖到网格底层
		}
		//启动每个worker，并将它们分配到不同的goroutine中
		wg.Add(1)
		go func(startY, endY int) {
			defer wg.Done()
			worker(startY, endY, currWorld, nextWorld, width, resultCh, &mu)
		}(startY, endY)
	}

	go func() {
		wg.Wait()
		close(resultCh)
	}()

	var aliveCells []stdstruct.Cell
	for cells := range resultCh {
		aliveCells = append(aliveCells, cells...)
	}

	//更新结果
	res.World = nextWorld
	res.AliveCells = aliveCells

	//// Iterate over each cell in the world
	//for y := 0; y < height; y++ {
	//	for x := 0; x < width; x++ {
	//
	//		globalY := req.StartY + y
	//		globalX := req.StartX + x
	//		// Count the live neighbors
	//		liveNeighbors := countLiveNeighbors(currWorld, globalY, globalX, req.EndY, req.EndX)
	//		// Apply the Game of Life rules
	//		if currWorld[globalY][globalX] == 255 {
	//			// Cell is alive
	//			if liveNeighbors < 2 || liveNeighbors > 3 {
	//				nextWorld[y][x] = 0 // Cell dies
	//			} else {
	//				nextWorld[y][x] = 255 // Cell stays alive
	//				res.AliveCells = append(aliveCells, stdstruct.Cell{X: globalX, Y: globalY})
	//			}
	//		} else {
	//			// Cell is dead
	//			if liveNeighbors == 3 {
	//				nextWorld[y][x] = 255 // Cell becomes alive
	//				res.AliveCells = append(aliveCells, stdstruct.Cell{X: globalX, Y: globalY})
	//			} else {
	//				nextWorld[y][x] = 0 // Cell stays dead
	//			}
	//		}
	//	}
	//}
	//res.World = nextWorld
	return nil

}

// shutting down the server when k is pressed
func (s *GameOfLife) ShutDown(req *stdstruct.ShutRequest, res *stdstruct.ShutResponse) (err error) {
	fmt.Println("Shutting down the server")
	os.Exit(0)
	return nil
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&GameOfLife{})
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	fmt.Println("Server Start, Listening on " + listener.Addr().String())
	rpc.Accept(listener)
}
