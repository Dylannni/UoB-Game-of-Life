package stdstruct

type Cell struct {
	X, Y int
}

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
	World [][]byte
	Threads int
}

type GameResponse struct {
	World [][]byte
}

// type ServerAddress struct {
// 	Address string
// 	Port    string
// }

// type InitRequest struct {
// 	StartX int
// 	EndX   int
// 	StartY int
// 	EndY   int
// 	World  [][]byte
// 	// ExtendedSlice  [][]byte
// 	Threads int
// 	PreviousServer ServerAddress
// 	NextServer     ServerAddress
// }

// type InitResponse struct {
// }

type InitRequest struct {
	// StartX int
	// EndX   int
	// StartY int
	// EndY   int
	World  [][]byte
	// // ExtendedSlice  [][]byte
	// Threads int
	// PreviousServer ServerAddress
	// NextServer     ServerAddress
}

type InitResponse struct {
}

type SliceRequest struct {
	StartX int
	EndX   int
	StartY int
	EndY   int
	Slice  			[][]byte
	ExtendedSlice  	[][]byte
	PreviousServer 	string
	NextServer		string
}

type SliceResponse struct {
	Slice [][]byte
}

type HaloRequest struct {
}

type HaloResponse struct {
	HaloLine []byte
}
