package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	"sync"

	"uk.ac.bris.cs/gameoflife/stdstruct"
)

type Broker struct {
	serverList []*rpc.Client // List of controller addresses
	// mu         sync.Mutex
}

type ServerAddress struct {
	Address string
	Port    string
}

var NodesList = [...]ServerAddress{
	{Address: "54.175.210.39", Port: "8081"},
	{Address: "54.91.189.221", Port: "8082"},
	// {Address: "localhost", Port: "8083"},
	// {Address: "localhost", Port: "8084"},
	// {Address: "localhost", Port: "8085"},
}

func (b *Broker) initializeNodes() {
	numNodes := len(NodesList)
	if len(b.serverList) == 0 {
		b.serverList = make([]*rpc.Client, 0, numNodes)
		connectedNode := 0
		for i := range NodesList {
			address := NodesList[i].Address + ":" + NodesList[i].Port
			server, nodeErr := rpc.Dial("tcp", address)
			if nodeErr == nil {
				connectedNode += 1
				b.serverList = append(b.serverList, server)
				fmt.Println("Connected to node:", address)
			} else {
				fmt.Println("Failed to connect to node:", address, "Error:", nodeErr)
			}
			if connectedNode == numNodes {
				break
			}
		}
		// numNodes = connectedNode // Remove???
	}
}

// RunGol distributes the game world to controllers and collects results
func (b *Broker) RunGol(req *stdstruct.GameRequest, res *stdstruct.GameResponse) error {
	b.initializeNodes()

	numServers := len(b.serverList)
	if numServers == 0 {
		return fmt.Errorf("no available server")
	}

	height := len(req.World)
	// width := len(req.World[0])
	sliceHeight := height / numServers

	var wg sync.WaitGroup
	results := make([][][]byte, numServers)
	errors := make([]error, numServers)

	for i, client := range b.serverList {
		startY := i * sliceHeight
		endY := startY + sliceHeight
		if i == numServers-1 {
			endY = height // Ensure last slice includes any remaining rows
		}
		slice := req.World[startY:endY]

		wg.Add(1)
		go func(i int, addr *rpc.Client, slice [][]byte, startY, endY int) {
			defer wg.Done()
			sliceReq := stdstruct.SliceRequest{
				StartY: startY,
				EndY:   endY,
				World:  slice,
			}
			var sliceRes stdstruct.SliceResponse
			err := client.Call("GameOfLife.ProcessSlice", sliceReq, &sliceRes)
			if err != nil {
				fmt.Println("Error processing slice:", err)
				errors[i] = err
				return
			}
			results[i] = sliceRes.World
		}(i, client, slice, startY, endY)
	}

	wg.Wait()

	for _, err := range errors {
		if err != nil {
			return err
		}
	}

	// Merge results
	newWorld := make([][]byte, 0)
	for _, part := range results {
		newWorld = append(newWorld, part...)
	}
	res.World = newWorld
	return nil
}

func (b *Broker) ShutDown(_ *stdstruct.ShutRequest, _ *stdstruct.ShutResponse) error {
	fmt.Println("Shutting down the broker")
	// Optionally, send shutdown signals to controllers
	os.Exit(0)
	return nil
}

func main() {
	rpc.Register(&Broker{})
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	rpc.Register(&Broker{})
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		panic(err)
	}
	defer listener.Close()
	fmt.Println("Broker started, listening on", listener.Addr().String())
	rpc.Accept(listener)
}
