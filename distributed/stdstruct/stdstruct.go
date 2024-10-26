package stdstruct

type Cell struct {
	X, Y int
}

type Request struct {
	StartX int
	EndX   int
	StartY int
	EndY   int
	World  [][]byte
}

type Response struct {
	World      [][]byte
	AliveCells []Cell
}
