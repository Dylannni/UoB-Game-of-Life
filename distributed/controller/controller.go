package main

import (
	//"errors"

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
	p := server.Params{
		Turns:       0,
		Threads:     0,
		ImageHeight: req.EndY,
		ImageWidth:  req.EndX,
	}
	world := server.InitWorld(req.EndY, req.EndX)
	for y := 0; y < req.EndY; y++ {
		for x := 0; x < req.EndX; x++ {
			world[y][x] = req.World[y][x]
		}
	}
	nextSate := server.CalculateNextState(req.StartY, req.EndY, 0, req.EndX, p, world)

	res.World = nextSate
	return nil
}

// shutting down the server when k is pressed
func (s *GameOfLife) ShutDown(req *stdstruct.ShutRequest, res *stdstruct.ShutResponse) (err error) {
	fmt.Println("Shutting down the server")
	os.Exit(0)
	return nil
}

func main() {
	workerPort := "8031"
	rpc.Register(&GameOfLife{})
	listener, err := net.Listen("tcp", ":"+workerPort)
	//listener, err := net.Listen("tcp", ":"+*worker2Port)
	if err != nil {
		panic(err)
	}

	defer listener.Close()
	fmt.Println("Worker is listening on port", workerPort)
	go rpc.Accept(listener)

}
