package stubs

var ReverseHandler = "SecretStringOperations.Reverse"
var PremiumReverseHandler = "SecretStringOperations.FastReverse"
var EvolveHandler = "GameOfLifeOperations.Evolve"

type Response struct {
	Message string
}

type Request struct {
	Message string
}

type Params struct {
	Turns       int
	Threads     int
	ImageWidth  int
	ImageHeight int
}

//The game board -> 2D slice It represents the current state of the Game of Life grid
type Board [][]byte
