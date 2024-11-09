package main

import (
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"os"

	"uk.ac.bris.cs/gameoflife/client/util"
	"uk.ac.bris.cs/gameoflife/stdstruct"
)

type Broker struct {
	serverList     []*rpc.Client // List of controller addresses
	connectedNodes int           // number of connected node
}

type ServerAddress struct {
	Address string
	Port    string
}

var NodesList = [...]ServerAddress{
	{Address: "54.205.187.254", Port: "8031"},
	{Address: "54.205.156.108", Port: "8032"},
	{Address: "107.23.213.173", Port: "8033"},
	{Address: "18.234.56.191", Port: "8034"},
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

	var outChannels []chan [][]byte
	var flippedCellsCh []chan []util.Cell // could use one single channel

	for i, server := range b.serverList {
		startY := i * sliceHeight
		endY := startY + sliceHeight

		slice := req.World[startY:endY]

		preNodeIndex := (i - 1 + b.connectedNodes) % b.connectedNodes
		nextNodeIndex := (i + 1 + b.connectedNodes) % b.connectedNodes

		err := server.Call("GameOfLife.Init", stdstruct.InitRequest{
			Height:         endY - startY,
			World:          slice,
			PreviousServer: NodesList[preNodeIndex].Address + ":" + NodesList[preNodeIndex].Port,
			NextServer:     NodesList[nextNodeIndex].Address + ":" + NodesList[nextNodeIndex].Port,
		}, &stdstruct.InitResponse{})
		if err != nil {
			fmt.Println("Error init GameOfLife:", err)
		}
	}

	for i, server := range b.serverList {
		startY := i * sliceHeight
		endY := startY + sliceHeight

		slice := req.World[startY:endY]

		sliceReq := stdstruct.SliceRequest{
			StartX: 0,
			EndX:   width,
			StartY: startY,
			EndY:   endY,
			Slice:  slice,
		}
		outChannel := make(chan [][]byte)
		flippedCellCh := make(chan []util.Cell)
		outChannels = append(outChannels, outChannel)
		flippedCellsCh = append(flippedCellsCh, flippedCellCh)
		go runAWSnode(server, sliceReq, outChannel, flippedCellCh)
	}

	// Merge results
	newWorld := make([][]byte, 0, height)
	for i := 0; i < numServers; i++ {
		newWorld = append(newWorld, <-outChannels[i]...)
	}

	// Merge flipped cells
	var cellFlipped []util.Cell
	for i := 0; i < numServers; i++ {
		cellFlipped = append(cellFlipped, <-flippedCellsCh[i]...)
	}

	res.World = newWorld
	res.FlippedCells = cellFlipped
	return nil
}

func runAWSnode(server *rpc.Client, sliceReq stdstruct.SliceRequest, out chan<- [][]byte, flippedCellCh chan<- []util.Cell) {
	var sliceRes stdstruct.SliceResponse
	err := server.Call("GameOfLife.CalculateNextTurn", sliceReq, &sliceRes)

	if err != nil {
		fmt.Println("Error processing slice:", err)
	}
	out <- sliceRes.Slice
	flippedCellCh <- sliceRes.FlippedCells
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
