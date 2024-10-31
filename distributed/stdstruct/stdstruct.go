package stdstruct

//import "uk.ac.bris.cs/gameoflife/stdstruct"

var Publish = "Broker.Publish"
var Subscribe = "Broker.Subscribe"

type Cell struct {
	X, Y int
}

type CalRequest struct {
	StartX    int
	EndX      int
	StartY    int
	EndY      int
	World     [][]byte
	TurnCount int
	Section   int
}

type CalResponse struct {
	StartX     int
	EndX       int
	StartY     int
	EndY       int
	World      [][]byte
	AliveCells []Cell
}

type ShutRequest struct {
}

type ShutResponse struct {
}

type SubscriptionRequest struct {
	Topic          string // subcribe topic
	CallbackMethod string // The callback method to handle data
	ClientAddress  string // subscribing client address
}

type SubscriptionResponse struct {
	Status string // A message confirming the subscription success
}

type ChannelRequest struct {
	Topic  string
	Buffer int
}

type Status struct {
	Message string
}

type Subscription struct {
	Topic          string
	FactoryAddress string
	Callback       string
}

type PublishRequest struct {
	Topic   string
	Request CalRequest
}

type ResultRequest struct {
	Topic string
}
type ResultResponse struct {
	Status  string
	Results []CalResponse
}
