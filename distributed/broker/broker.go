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
	connectedNodes int 		 // number of connected node
}

type ServerAddress struct {
	Address string
	Port    string
}

var NodesList = [...]ServerAddress{
	{Address: "44.211.202.200", Port: "8031"},
	{Address: "3.83.11.239", Port: "8032"},
	{Address: "3.89.206.35", Port: "8033"},
	{Address: "23.23.24.46", Port: "8034"},
}

func (b *Broker) initNodes() {
	numNodes := len(NodesList)
	if len(b.serverList) == 0 {
		b.serverList = make([]*rpc.Client, 0, numNodes)
		b.connectedNodes = 0
		for i := range NodesList {
			address := NodesList[i].Address + ":" + NodesList[i].Port
			server, nodeErr := rpc.Dial("tcp", address)
			if nodeErr == nil {
				b.connectedNodes += 1
				b.serverList = append(b.serverList, server)
				fmt.Println("Connected to node:", address)
			} else {
				fmt.Println("Failed to connect to node:", address, "Error:", nodeErr)
			}
			if b.connectedNodes == numNodes {
				break
			}
		}
	}
}

// RunGol distributes the game world to controllers and collects results
func (b *Broker) RunGol(req *stdstruct.GameRequest, res *stdstruct.GameResponse) error {
	b.initNodes()

	numServers := len(b.serverList)
	if numServers == 0 {
		return fmt.Errorf("no available server")
	}

	height := len(req.World)
	width := len(req.World[0])
	sliceHeight := height / numServers

	// results := make([][][]byte, numServers)
	var outChannels []chan [][]byte
	// errors := make([]error, numServers)

	for i, server := range b.serverList {
		startY := i * sliceHeight
		endY := startY + sliceHeight

		slice := req.World[startY:endY]

		err := server.Call("GameOfLife.Init", stdstruct.InitRequest{World: slice}, &stdstruct.InitResponse{})
		if err != nil {
			fmt.Println("Error init GameOfLife:", err)
		}
	}


	for i, server := range b.serverList {
		startY := i * sliceHeight
		endY := startY + sliceHeight

		slice := req.World[startY:endY]

		// var extendedSlice [][]byte
		// if startY == 0 {
		// 	// adding the last row of the last slice to the top
		// 	extendedSlice = append([][]byte{req.World[height-1]}, slice...)
		// } else {
		// 	// adding the last row of the last slice to the top
		// 	extendedSlice = append([][]byte{req.World[startY-1]}, slice...)
		// }

		// if endY == height {
		// 	// 最后一块切片，添加第一行作为 Ghost Cell
		// 	extendedSlice = append(extendedSlice, req.World[0])
		// } else {
		// 	// adding the first row of next slice to the bottom
		// 	extendedSlice = append(extendedSlice, req.World[endY])
		// }

		preNodeIndex := (i-1+b.connectedNodes)%b.connectedNodes
		nextNodeIndex := (i+1+b.connectedNodes)%b.connectedNodes

		sliceReq := stdstruct.SliceRequest{
			StartX: 0,
			EndX: 	width,
			StartY: startY,
			EndY:   endY,
			Slice:  slice,
			PreviousServer: NodesList[preNodeIndex].Address+":"+NodesList[preNodeIndex].Port,
			NextServer:     NodesList[nextNodeIndex].Address+":"+NodesList[nextNodeIndex].Port,
			// ExtendedSlice:  extendedSlice,
		}
		outChannel := make(chan [][]byte)
		outChannels = append(outChannels, outChannel)
		go runAWSnode(server, sliceReq, outChannel)
		// preNodeIndex := (i-1+b.connectedNodes)%b.connectedNodes
		// nextNodeIndex := (i+1+b.connectedNodes)%b.connectedNodes

		// err := server.Call("GameOfLife.Init", stdstruct.InitRequest{
		// 	StartX: 0,
		// 	EndX: 	width,
		// 	StartY: startY,
		// 	EndY:   endY,
		// 	World:  slice,
		// 	Threads:        req.Threads,
		// 	PreviousServer: stdstruct.ServerAddress{NodesList[preNodeIndex].Address, NodesList[preNodeIndex].Port},
		// 	NextServer:     stdstruct.ServerAddress{NodesList[nextNodeIndex].Address, NodesList[nextNodeIndex].Port},
		// }, &stdstruct.InitResponse{})
		// if err != nil {
		// 	fmt.Println("Error init :", err)
		// }

		// outChannel := make(chan [][]byte)
		// outChannels = append(outChannels, outChannel)
		// go runAWSnode(server, sliceReq, outChannel)
	}

	// for i, server := range b.serverList {
	// 	startY := i * sliceHeight
	// 	endY := startY + sliceHeight

	// 	slice := req.World[startY:endY]

	// 	var extendedSlice [][]byte
	// 	if startY == 0 {
	// 		// adding the last row of the last slice to the top
	// 		extendedSlice = append([][]byte{req.World[height-1]}, slice...)
	// 	} else {
	// 		// adding the last row of the last slice to the top
	// 		extendedSlice = append([][]byte{req.World[startY-1]}, slice...)
	// 	}

	// 	if endY == height {
	// 		// 最后一块切片，添加第一行作为 Ghost Cell
	// 		extendedSlice = append(extendedSlice, req.World[0])
	// 	} else {
	// 		// adding the first row of next slice to the bottom
	// 		extendedSlice = append(extendedSlice, req.World[endY])
	// 	}

	// 	sliceReq := stdstruct.SliceRequest{
	// 		StartX: 0,
	// 		EndX: 	width,
	// 		StartY: startY,
	// 		EndY:   endY,
	// 		Slice:  slice,
	// 		ExtendedSlice:  extendedSlice,
	// 	}
	// 	outChannel := make(chan [][]byte)
	// 	outChannels = append(outChannels, outChannel)
	// 	go runAWSnode(server, sliceReq, outChannel)
	// }

	// Merge results
	newWorld := make([][]byte, 0, height)
	for i := 0; i < numServers; i++ {
		newWorld = append(newWorld, <-outChannels[i]...)
	}

	res.World = newWorld
	return nil
}

func runAWSnode(server *rpc.Client, sliceReq stdstruct.SliceRequest, out chan<- [][]byte) {
	var sliceRes stdstruct.SliceResponse
	err := server.Call("GameOfLife.CalculateNextTurn", sliceReq, &sliceRes)

	if err != nil {
		fmt.Println("Error processing slice:", err)
	}
	out <- sliceRes.Slice
}

func (b *Broker) ShutDown(_ *stdstruct.ShutRequest, _ *stdstruct.ShutResponse) (err error) {
	fmt.Println("Shutting down the broker")
	// Optionally, send shutdown signals to controllers
	for _, server := range b.serverList {
		err = server.Call("GameOfLife.ShutDown", stdstruct.ShutRequest{}, stdstruct.ShutResponse{})
	}
	fmt.Println("Broker Stopped")
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
