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
	{Address: "52.91.216.135", Port: "8031"},
	{Address: "3.80.106.139", Port: "8032"},
	{Address: "54.159.3.141", Port: "8033"},
	{Address: "50.17.117.83", Port: "8034"},
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

	var flippedCellsCh []chan []util.Cell // list of flipped cells that yet to merge

	for i, server := range b.serverList {
		startY := i * sliceHeight
		endY := startY + sliceHeight

		// slice of the world that needs to calculated
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

		sliceReq := stdstruct.SliceRequest{
			StartX:  0,
			EndX:    width,
			StartY:  startY,
			EndY:    endY,
			Threads: req.Threads,
			// ExtendedSlice: extendedSlice,
		}
		flippedCellCh := make(chan []util.Cell)
		flippedCellsCh = append(flippedCellsCh, flippedCellCh)
		go runAWSnode(server, sliceReq, flippedCellCh)
	}
	newWorld := req.World // no potential race condition, no need to copy

	// Merge flipped cells
	var cellFlippeds []util.Cell
	for i := 0; i < numServers; i++ {
		cellFlippeds = append(cellFlippeds, <-flippedCellsCh[i]...)
	}

	for _, flippedCell := range cellFlippeds {
		if newWorld[flippedCell.Y][flippedCell.X] == 255 {
			newWorld[flippedCell.Y][flippedCell.X] = 0
		} else {
			newWorld[flippedCell.Y][flippedCell.X] = 255
		}
	}
	res.World = newWorld
	res.FlippedCells = cellFlippeds
	return nil
}

func runAWSnode(server *rpc.Client, sliceReq stdstruct.SliceRequest, flippedCellCh chan<- []util.Cell) {
	var sliceRes stdstruct.SliceResponse
	err := server.Call("GameOfLife.NextTurn", sliceReq, &sliceRes)

	if err != nil {
		fmt.Println("Error processing slice:", err)
	}
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
