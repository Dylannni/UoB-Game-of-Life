package stdstruct

import "uk.ac.bris.cs/gameoflife/client/util"


type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type ShutRequest struct {
}

type ShutResponse struct {
}

type RegisterRequest struct {
	Address string
}

type RegisterResponse struct {
}

type GameRequest struct {
	World   [][]byte
	Threads int
}

type GameResponse struct {
	World        [][]byte
	FlippedCells []util.Cell
}

type InitRequest struct {
	World          [][]byte
	Height         int
	Threads        int
	PreviousServer string
	NextServer     string
}

type InitResponse struct {
}

type SliceRequest struct {
	StartX int
	EndX   int
	StartY int
	EndY   int
	Slice  [][]byte
}

type SliceResponse struct {
	Slice [][]byte
}

type HaloRequest struct {
}

type HaloResponse struct {
	HaloLine []byte
}
