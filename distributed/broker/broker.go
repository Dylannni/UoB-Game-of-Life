package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"sync"

	"uk.ac.bris.cs/gameoflife/stdstruct"
)

// store each topic task
var (
	topics     = make(map[string]chan stdstruct.CalRequest)
	responseCh = make(map[string][]chan stdstruct.CalResponse, len(workers))
	workers    = []string{"3.85.22.253:8031", "52.87.242.13:8032"}
	topicmx    sync.RWMutex
)

func InitWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

// Create a new topic as a buffered channel.
func newTopic(topic string, buflen int) {
	topicmx.Lock()
	defer topicmx.Unlock()
	if _, ok := topics[topic]; !ok {
		fmt.Printf("Creating new topic: %s with buffer length: %d\n", topic, buflen)
		topics[topic] = make(chan stdstruct.CalRequest, buflen)
		responseCh[topic] = make([]chan stdstruct.CalResponse, len(workers))
		for i := range workers {
			responseCh[topic][i] = make(chan stdstruct.CalResponse, 1)
		}
	}
}

// add task to the specific Topic task queue
func publish(topic string, request stdstruct.CalRequest) (err error) {
	topicmx.RLock()
	responseChannels, ok := responseCh[topic]
	topicmx.RUnlock()
	if !ok {
		fmt.Printf("Topic %s not found\n", topic)
		return errors.New("topic not found")
	}

	fmt.Printf("Publishing request to topic: %s\n", topic)

	var wg sync.WaitGroup
	wg.Add(len(workers))

	for i, workerAddress := range workers {
		go func(workerAddress string, req stdstruct.CalRequest, idx int) {
			defer wg.Done()

			fmt.Printf("Attempting to connect to worker: %s\n", workerAddress)
			client, err := rpc.Dial("tcp", workerAddress)
			if err != nil {
				fmt.Printf("Error connecting to worker %s: %v\n", workerAddress, err)
				return
			}
			defer client.Close()

			response := new(stdstruct.CalResponse)
			err = client.Call("GameOfLife.CalculateNextTurn", req, response)
			if err != nil {
				fmt.Printf("Error during worker calculation at %s: %v\n", workerAddress, err)
				return
			}

			responseChannels[idx] <- *response
		}(workerAddress, request, i)
	}

	wg.Wait()
	return nil
}

// // The task is continuously fetched from the specified topic channel, processed, and the result is sent to the server through RPC.
// // process the task from server
// func subscriberLoop(topic string, resquestCh chan stdstruct.CalRequest, client *rpc.Client, callback string) {
// 	for {
// 		job := <-resquestCh
// 		response := new(stdstruct.CalResponse)
// 		err := client.Call(callback, job, response)
// 		if err != nil {
// 			fmt.Println(err)
// 			//Place the unfulfilled job back on the topic channel.
// 			resquestCh <- job
// 			client.Close()
// 			break
// 		} else {
// 			topicmx.RLock()
// 			responseChannel, exists := responseCh[topic]
// 			topicmx.RUnlock()
// 			if exists {
// 				responseChannel <- *response
// 			}
// 		}
// 	}
// }

// // subscribe specific job to a worker,Enables the node to fetch tasks from this topic and process them
// func subscribe(topic string, workerAddress string, callback string) (err error) {
// 	topicmx.RLock()
// 	requestCh, exists := topics[topic]
// 	topicmx.RUnlock()

// 	if !exists {
// 		return errors.New("topic not found")
// 	}

// 	client, err := rpc.Dial("tcp", workerAddress)
// 	if err != nil {
// 		return err
// 	}
// 	go subscriberLoop(topic, requestCh, client, callback)
// 	return nil
// }

// collect the results
func collectResponses(topic string) (res []stdstruct.CalResponse, err error) {
	topicmx.RLock()
	responseChannel, ok := responseCh[topic]
	topicmx.RUnlock()
	if !ok {
		fmt.Printf("Response channel for topic %s not found\n", topic)
		return nil, errors.New("not found")
	}
	expectedResponses := len(workers)
	fmt.Printf("Expecting %d responses for topic: %s\n", expectedResponses, topic)

	var wg sync.WaitGroup
	var mu sync.Mutex
	wg.Add(expectedResponses)

	for i := 0; i < expectedResponses; i++ {
		go func(i int) {
			defer wg.Done()
			result := <-responseChannel[i] // Blocking until response is received

			mu.Lock() // Lock to safely append to the result slice
			res = append(res, result)
			mu.Unlock()

			fmt.Printf("Received response from worker %d for topic: %s\n", i+1, topic)
		}(i)
	}
	wg.Wait()
	fmt.Printf("Finished collecting responses for topic: %s\n", topic)
	return res, nil
}

type Broker struct {
	shutdownChan chan struct{}
}

// create a new channel for the publishing and proccessing the task
func (b *Broker) CreateChannel(req stdstruct.ChannelRequest, res *stdstruct.Status) (err error) {
	fmt.Printf("Received request to create channel: %s\n", req.Topic)
	newTopic(req.Topic, req.Buffer)
	res.Message = "Channel created"
	return
}

// func (b *Broker) Subscribe(req stdstruct.Subscription, res *stdstruct.Status) (err error) {
// 	err = subscribe(req.Topic, req.FactoryAddress, req.Callback)
// 	return err
// }

func (b *Broker) Publish(req stdstruct.PublishRequest, res *stdstruct.Status) (err error) {
	fmt.Printf("Received publish request for topic: %s\n", req.Topic)
	err = publish(req.Topic, req.Request)
	return err
}

func (b *Broker) CollectResponses(req stdstruct.ResultRequest, res *stdstruct.ResultResponse) (err error) {
	fmt.Printf("Received request to collect responses for topic: %s\n", req.Topic)
	result, _ := collectResponses(req.Topic)

	finalWorld := InitWorld(req.ImageHeight, req.ImageWidth)

	var finalAliveCells []stdstruct.Cell

	for _, res := range result {
		for y := res.StartY; y < res.EndY; y++ {
			for x := res.StartX; x < res.EndX; x++ {
				finalWorld[y][x] = res.World[y][x]
			}
		}
		finalAliveCells = append(finalAliveCells, res.AliveCells...)
	}
	res.World = finalWorld
	res.AliveCells = finalAliveCells
	res.Results = result

	return nil

}

func (b *Broker) ShutDownBroker(req stdstruct.ShutRequest, res stdstruct.ShutResponse) (err error) {
	fmt.Println("Shutting down the broker")
	close(b.shutdownChan)
	return nil
}
func main() {
	pAddr := flag.String("port", "8030", "Port to listen on")
	flag.Parse()
	broker := &Broker{shutdownChan: make(chan struct{})}

	rpc.Register(broker)
	listener, err := net.Listen("tcp", ":"+*pAddr)
	if err != nil {
		fmt.Printf("Error starting broker: %v\n", err)
		return
	}
	defer listener.Close()
	fmt.Printf("Broker is listening on port %s\n", *pAddr)
	go func() { rpc.Accept(listener) }()

	<-broker.shutdownChan
	fmt.Println("Broker has been shut down")
}
