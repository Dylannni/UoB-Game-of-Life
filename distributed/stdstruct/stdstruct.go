package stdstruct

type Cell struct {
	X, Y int
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
