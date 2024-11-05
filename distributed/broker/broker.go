package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"
	// "sync"

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
	{Address: "34.239.0.10", Port: "8031"},
	{Address: "44.210.94.134", Port: "8032"},
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
	width := len(req.World[0])
	sliceHeight := height / numServers

	// var wg sync.WaitGroup
	results := make([][][]byte, numServers)
	errors := make([]error, numServers)

	for i, client := range b.serverList {

		// var startY, endY int
		startY := i * sliceHeight
		endY := startY + sliceHeight

		// if i == 0 {
		// 	startY = 0
		// 	endY = sliceHeight + 4
		// } else if i == numServers-1 {
		// 	// go worker(i*workerHeight-4, (i+1)*workerHeight, 0, width, immutableData, out[i])
		// 	startY = i*sliceHeight - 4
		// 	endY = (i + 1) * sliceHeight
		// } else {
		// 	// go worker(i*workerHeight-2, (i+1)*workerHeight+2, 0, width, immutableData, out[i])

		// 	startY = i*sliceHeight - 2
		// 	endY = (i+1)*sliceHeight + 2
		// }

		// if i == numServers-1 {
		// 	endY = height // Ensure last slice includes any remaining rows
		// }

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

		// var slice [][]byte
		// if startY == 0 {
		// 	slice = req.World[startY : endY+2]
		// } else if endY == height {
		// 	slice = req.World[startY-2 : endY]
		// } else {
		// 	slice = req.World[startY-1 : endY+1]
		// }

		// var mutex sync.Mutex

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

		// wg.Add(1)
		// // var mutex sync.Mutex
		// go func(i int, addr *rpc.Client, slice [][]byte, world [][]byte, startY, endY, startX, endX int) {
		// 	defer wg.Done()
		// 	sliceReq := stdstruct.SliceRequest{
		// 		StartX: startX,
		// 		EndX: 	endX,
		// 		StartY: startY,
		// 		EndY:   endY,
		// 		World:  world,
		// 		Slice:  slice,
		// 	}
		// 	var sliceRes stdstruct.SliceResponse
		// 	err := client.Call("GameOfLife.CalculateNextTurn", sliceReq, &sliceRes)
		// 	if err != nil {
		// 		fmt.Println("Error processing slice:", err)
		// 		errors[i] = err
		// 		return
		// 	}
		// 	fmt.Printf("Worker %d processed rows %d to %d\n", i, startY, endY)
		// 	// mutex.Lock()
		// 	results[i] = sliceRes.Slice
		// 	// mutex.Unlock()
		// }(i, client, extendedSlice, req.World, startY, endY, 0, width)
	}

	// wg.Wait()

	// for _, err := range errors {
	// 	if err != nil {
	// 		return err
	// 	}
	// }

	// Merge results
	newWorld := make([][]byte, 0, height)
	for i := 0; i < numServers; i++ {
		// slice := results[i]
		// if i == 0 {
		// 	// slice = slice[1:]
		// 	slice = slice[:len(slice)-1]
		// } else if i == numServers-1 {
		// 	slice = slice[1:]
		// 	// slice = slice[:len(slice)-1]
		// } else {
		// 	slice = slice[1 : len(slice)-1]
		// }

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
