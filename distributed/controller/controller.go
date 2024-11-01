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
	if req.StartY < 0 || req.EndY > len(req.World) || req.StartY > req.EndY {
		panic(fmt.Sprintf("Invalid StartY: %d, EndY: %d, WorldHeight: %d", req.StartY, req.EndY, len(req.World)))
	}
	if req.StartX < 0 || req.EndX > len(req.World[0]) || req.StartX > req.EndX {
		panic(fmt.Sprintf("Invalid StartX: %d, EndX: %d, WorldWidth: %d", req.StartX, req.EndX, len(req.World[0])))
	}
	p := server.Params{
		Turns:       0,
		Threads:     0,
		ImageHeight: len(req.World),
		ImageWidth:  len(req.World[0]),
	}
	fmt.Println("[DEBUG] Starting CalculateNextState")
	nextSate, aliveCells := server.CalculateNextState(req.StartY, req.EndY, req.StartX, req.EndX, p, req.World)

	req.World = nextSate
	res.AliveCells = aliveCells
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
