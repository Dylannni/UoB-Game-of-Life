package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"

	"uk.ac.bris.cs/gameoflife/stdstruct"
)

type Broker struct {
	serverList []*rpc.Client // List of controller addresses
}

type ServerAddress struct {
	Address string
	Port    string
}

var NodesList = [...]ServerAddress{
	{Address: "44.203.150.15", Port: "8031"},
	{Address: "54.86.171.180", Port: "8032"},
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
	width := len(req.World[0])
	sliceHeight := height / numServers

	results := make([][][]byte, numServers)
	errors := make([]error, numServers)

	for i, client := range b.serverList {

		startY := i * sliceHeight
		endY := startY + sliceHeight

		 // 提取当前切片
		slice := req.World[startY:endY]

		// 声明并初始化 extendedSlice
		var extendedSlice [][]byte
		if startY == 0 {
			// adding the last row of the last slice to the top
			extendedSlice = append([][]byte{req.World[height-1]}, slice...)
		} else {
			// adding the last row of the last slice to the top
			extendedSlice = append([][]byte{req.World[startY-1]}, slice...)
		}

		if endY == height {
			// 最后一块切片，添加第一行作为 Ghost Cell
			extendedSlice = append(extendedSlice, req.World[0])
		} else {
			// adding the first row of next slice to the bottom
			extendedSlice = append(extendedSlice, req.World[endY])
		}

		sliceReq := stdstruct.SliceRequest{
			StartX: 0,
			EndX: 	width,
			StartY: startY,
			EndY:   endY,
			Slice:  slice,
			ExtendedSlice:  extendedSlice,
		}
		var sliceRes stdstruct.SliceResponse
		err := client.Call("GameOfLife.CalculateNextTurn", sliceReq, &sliceRes)
		if err != nil {
			fmt.Println("Error processing slice:", err)
			errors[i] = err
		}
		fmt.Printf("Worker %d processed rows %d to %d\n", i, startY, endY)

		results[i] = sliceRes.Slice
	}

	// Merge results
	newWorld := make([][]byte, 0, height)
	for i := 0; i < numServers; i++ {
		newWorld = append(newWorld, results[i]...)
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
