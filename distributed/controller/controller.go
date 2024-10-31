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
	brokerAddr := flag.String("broker", "3.95.198.0:8030", "Broker address")
	workerIP := flag.String("ip", "98.81.84.228", "44.201.136.20")
	workerPort := flag.String("port", "8031", "Worker port")
	flag.Parse()
	rpc.Register(&GameOfLife{})
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%s", *workerIP, *workerPort))
	//listener, err := net.Listen("tcp", ":"+*worker2Port)
	if err != nil {
		panic(err)

	}

	defer listener.Close()
	go rpc.Accept(listener)

	// connect to broker
	client, err := rpc.Dial("tcp", *brokerAddr)
	if err != nil {
		fmt.Println("Error connecting to broker:", err)
		os.Exit(1)
	}
	defer client.Close()

	// Construct the worker address
	workerAddr := fmt.Sprintf("%s:%s", *workerIP, *workerPort)

	// Subscribe to the broker
	var subRes stdstruct.Status
	subReq := stdstruct.Subscription{
		Topic:          "game of life task",
		FactoryAddress: workerAddr,
		Callback:       "GameOfLife.CalculateNextTurn",
	}
	client.Call("Broker.Subscribe", subReq, &subRes)
}
