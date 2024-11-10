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
	serverList []*rpc.Client // List of controller addresses
}

type ServerAddress struct {
	Address string
	Port    string
}

var NodesList = [...]ServerAddress{
	{Address: "98.81.254.164", Port: "8031"},
	{Address: "35.173.244.251", Port: "8032"},
	{Address: "54.82.113.89", Port: "8033"},
	{Address: "54.157.37.183", Port: "8034"},
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

	var flippedCellsCh []chan []util.Cell // list of flipped cells that yet to merge

	for i, server := range b.serverList {
		startY := i * sliceHeight
		endY := startY + sliceHeight

		// slice of the world that needs to calculated
		slice := req.World[startY:endY]

		// extendedSlice is the slice with two (top, bottom) halo line
		var extendedSlice [][]byte
		if startY == 0 {
			// adding the last row of the world to the top
			extendedSlice = append([][]byte{req.World[height-1]}, slice...)
		} else {
			// adding the last row of the last slice to the top
			extendedSlice = append([][]byte{req.World[startY-1]}, slice...)
		}

		if endY == height {
			// adding the first line of the world to the bottom
			extendedSlice = append(extendedSlice, req.World[0])
		} else {
			// adding the first line of next slice to the bottom
			extendedSlice = append(extendedSlice, req.World[endY])
		}

		sliceReq := stdstruct.SliceRequest{
			StartX:        0,
			EndX:          width,
			StartY:        startY,
			EndY:          endY,
			ExtendedSlice: extendedSlice,
		}
		flippedCellCh := make(chan []util.Cell)
		flippedCellsCh = append(flippedCellsCh, flippedCellCh)
		go runAWSnode(server, sliceReq, flippedCellCh)
	}

	// newWorld := make([][]byte, height)
	// for i := range newWorld {
	// 	newWorld[i] = make([]byte, width)
	// 	copy(newWorld[i], req.World[i])
	// }
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
	err := server.Call("GameOfLife.CalculateNextTurn", sliceReq, &sliceRes)

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
