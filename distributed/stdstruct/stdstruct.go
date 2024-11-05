package stdstruct

// Use by distributor
type Cell struct {
	X, Y int
}

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

type CalRequest struct {
	StartX int
	EndX   int
	StartY int
	EndY   int
	World  [][]byte
}

type CalResponse struct {
	World      [][]byte
	AliveCells []Cell
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
	World [][]byte
}

type GameResponse struct {
	World [][]byte
}

type SliceRequest struct {
	StartY int
	EndY   int
	World  [][]byte
}

type SliceResponse struct {
	World [][]byte
}
