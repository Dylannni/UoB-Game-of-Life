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

type SliceRequest struct {
	StartX        int
	EndX          int
	StartY        int
	EndY          int
	Threads       int
	ExtendedSlice [][]byte
}

type SliceResponse struct {
	Slice        [][]byte
	FlippedCells []util.Cell
}
