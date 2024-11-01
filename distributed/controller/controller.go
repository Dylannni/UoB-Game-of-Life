package main

import (
	//"errors"
	"flag"
	"fmt"
	"os"

	//"fmt"

	"net"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/controller/server"

	"uk.ac.bris.cs/gameoflife/stdstruct"
)

type GameOfLife struct{}

func (s *GameOfLife) CalculateNextTurn(req *stdstruct.CalRequest, res *stdstruct.CalResponse) (err error) {
	fmt.Println("[DEBUG] Received CalculateNextTurn request")
	p := server.Params{
		Turns:       0,
		Threads:     0,
		ImageHeight: req.EndY - req.StartY,
		ImageWidth:  req.EndX - req.StartX,
	}
	subWorldHeight := req.EndY - req.StartY
	subWorldWidth := req.EndX - req.StartX

	subWorld := make([][]byte, subWorldHeight)
	for i := range subWorld {
		subWorld[i] = make([]byte, subWorldWidth)
		for j := 0; j < subWorldWidth; j++ {
			subWorld[i][j] = req.World[req.StartY+i][req.StartX+j]
		}
	}
	fmt.Println("[DEBUG] Starting CalculateNextState")
	nextSate, aliveCells := server.CalculateNextState(0, subWorldHeight, 0, subWorldWidth, p, subWorld)

	res.World = nextSate
	res.AliveCells = make([]stdstruct.Cell, len(aliveCells))

	// Update alive cell positions to the global coordinates
	for i, cell := range aliveCells {
		res.AliveCells[i] = stdstruct.Cell{
			X: req.StartX + cell.X,
			Y: req.StartY + cell.Y,
		}
	}
	res.StartX = req.StartX
	res.EndX = req.EndX
	res.StartY = req.StartY
	res.EndY = req.EndY
	fmt.Println("[DEBUG] Completed CalculateNextTurn")
	return nil
}

// shutting down the server when k is pressed
func (s *GameOfLife) ShutDown(req *stdstruct.ShutRequest, res *stdstruct.ShutResponse) (err error) {
	fmt.Println("Shutting down the server")
	os.Exit(0)
	return nil
}

func main() {
	workerPort := flag.String("port", "8031", "Worker port to listen on")
	flag.Parse()
	fmt.Println("[DEBUG] Registering GameOfLife RPC service")
	rpc.Register(&GameOfLife{})
	listener, err := net.Listen("tcp", ":"+*workerPort)
	//listener, err := net.Listen("tcp", ":"+*worker2Port)
	if err != nil {
		fmt.Printf("[ERROR] Failed to start listener on port %s: %v\n", *workerPort, err)
		panic(err)
	}

	defer listener.Close()
	fmt.Println("Worker is listening on port", *workerPort)
	fmt.Println("[DEBUG] Waiting for incoming connections...")
	rpc.Accept(listener)

}
