package main

import (
	//"errors"
	"flag"
	//"fmt"

	"net"
	"net/rpc"

	"uk.ac.bris.cs/gameoflife/controller/server"

	"uk.ac.bris.cs/gameoflife/stdstruct"
)

type GameOfLife struct{}

func (s *GameOfLife) CalculateNextTurn(req *stdstruct.Request, res *stdstruct.Response) (err error) {
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

func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&GameOfLife{})
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		panic(err)

	}
	defer listener.Close()
	rpc.Accept(listener)
}
