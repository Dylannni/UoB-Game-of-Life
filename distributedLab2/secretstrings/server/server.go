package main

import (
	"flag"
	"math/rand"
	"net"
	"net/rpc"
	"time"
	"uk.ac.bris.cs/distributed2/secretstrings/stubs"
	"uk.ac.bris.cs/gameoflife/util"
)

type Cell struct {
	X, Y int
}

type CellFlipped struct {
	CompletedTurns int
	Cell           Cell
}

type distributorChannels struct {
	completedTurns int
	events         chan CellFlipped
}

/** Super-Secret `reversing a string' method we can't allow clients to see. **/
//func ReverseString(s string, i int) string {
//	time.Sleep(time.Duration(rand.Intn(i)) * time.Second)
//	runes := []rune(s)
//	for i, j := 0, len(runes)-1; i < j; i, j = i+1, j-1 {
//		runes[i], runes[j] = runes[j], runes[i]
//	}
//	return string(runes)
//}
//
//type SecretStringOperations struct{}
//
//func (s *SecretStringOperations) Reverse(req stubs.Request, res *stubs.Response) (err error) {
//	if req.Message == "" {
//		err = errors.New("A message must be specified")
//		return
//	}
//
//	fmt.Println("Got Message: " + req.Message)
//	res.Message = ReverseString(req.Message, 10)
//	return
//}
//
//func (s *SecretStringOperations) FastReverse(req stubs.Request, res *stubs.Response) (err error) {
//	if req.Message == "" {
//		err = errors.New("A message must be specified")
//		return
//	}
//
//	res.Message = ReverseString(req.Message, 2)
//	return
//}

//Responsible for handling the Game of Life's turn updates
type GameOfLifeOperations struct{}

//record the state of the board after a specified turn
func (g *GameOfLifeOperations) Evolve(p stubs.Params, world *stubs.Board) error {
	//world -> initialise the state of the GOL grid
	//p -> store the configuration (eg: number of turns, grid dimensions)
	//track the number of turns completed
	completedTurns := 0

	//perform the evolution turns of the GOL
	for turn := 0; turn < p.Turns; turn++ {
		//计算下一回合的状态
		newWorld := calculateNextState(0, p.ImageHeight, 0, p.ImageWidth, p, *world)

		//更新world指向的内容
		*world = newWorld
		completedTurns++
	}
	//更新结果。将完成的回合数存储在world的Turn字段中
	//这一步假设在stubs.Board中，已经定义了一个turn字段来存储完成的回合数
	world.Turns = completedTurns

	//返回nil，表示函数执行成功
	return nil
}

//The game logic
// initialise world
func initWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

//Calculate the Alive Cells
func calculateAliveCells(p stubs.Params, world [][]byte) []Cell {
	var aliveCells []Cell
	for row := 0; row < p.ImageHeight; row++ {
		for col := 0; col < p.ImageWidth; col++ {
			if world[row][col] == 255 {
				aliveCells = append(aliveCells, Cell{X: col, Y: row})
			}
		}
	}
	return aliveCells
}

//calculate the neighbours
func countLiveNeighbors(world [][]byte, row, col, rows, cols int) int {
	neighbours := [8][2]int{
		{-1, -1}, {-1, 0}, {-1, 1}, {0, -1}, {0, 1}, {1, -1}, {1, 0}, {1, 1},
	}
	liveNeighbours := 0
	for _, n := range neighbours {
		newRow := (row + n[0] + rows) % rows
		newCol := (col + n[1] + cols) % cols
		if world[newRow][newCol] == 255 {
			liveNeighbours++
		}
	}
	return liveNeighbours
}

func calculateNextState(startY, endY, startX, endX int, p stubs.Params, world [][]byte, c distributorChannels) [][]byte {
	height := endY - startY
	width := endX - startX

	newWorld := make([][]byte, height)
	for i := range newWorld {
		newWorld[i] = make([]byte, width)
	}

	// Iterate over each cell in the world
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {

			globalY := startY + y
			globalX := startX + x
			// Count the live neighbors
			liveNeighbors := countLiveNeighbors(world, globalY, globalX, p.ImageHeight, p.ImageWidth)
			// Apply the Game of Life rules
			if world[globalY][globalX] == 255 {
				// Cell is alive
				if liveNeighbors < 2 || liveNeighbors > 3 {
					newWorld[y][x] = 0 // Cell dies
					c.events <- CellFlipped{CompletedTurns: c.completedTurns, Cell: util.Cell{X: globalX, Y: globalY}}
				} else {
					newWorld[y][x] = 255 // Cell stays alive
				}
			} else {
				// Cell is dead
				if liveNeighbors == 3 {
					newWorld[y][x] = 255 // Cell becomes alive
					c.events <- CellFlipped{CompletedTurns: c.completedTurns, Cell: util.Cell{X: globalX, Y: globalY}}
				} else {
					newWorld[y][x] = 0 // Cell stays dead
				}
			}
		}
	}
	return newWorld
}

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rand.Seed(time.Now().UnixNano())
	rpc.Register(&SecretStringOperations{})
	listener, _ := net.Listen("tcp", ":"+*pAddr)
	defer listener.Close()
	rpc.Accept(listener)
}
