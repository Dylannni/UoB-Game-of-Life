package main

import (
	"errors"
	"flag"
	"fmt"
	"net"
	"net/rpc"
	"sync"
	"time"

	"uk.ac.bris.cs/gameoflife/stdstruct"
)

// store each topic task
var (
	topics     = make(map[string]chan stdstruct.CalRequest)
	responseCh = make(map[string][]chan stdstruct.CalResponse, len(workers))
	workers    = []string{"100.27.228.161:8031", "3.84.31.117:8032"}
	topicmx    sync.RWMutex
)

func InitWorld(height, width int) [][]byte {
	world := make([][]byte, height)
	for i := range world {
		world[i] = make([]byte, width)
	}
	return world
}

func copyCalRequest(req stdstruct.CalRequest, startY, endY int) stdstruct.CalRequest {
	newWorld := make([][]byte, len(req.World))
	for i := range req.World {
		newWorld[i] = make([]byte, len(req.World[i]))
		copy(newWorld[i], req.World[i])
	}

	return stdstruct.CalRequest{
		StartX:    req.StartX,
		EndX:      req.EndX,
		StartY:    startY,
		EndY:      endY,
		World:     newWorld,
		TurnCount: req.TurnCount,
		Section:   req.Section,
	}
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
			responseCh[topic][i] = make(chan stdstruct.CalResponse, buflen)
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

	heightPerWorker := request.EndY / len(workers)
	var wg sync.WaitGroup

	for i, workerAddress := range workers {
		wg.Add(1)
		startY := i * heightPerWorker
		endY := (i + 1) * heightPerWorker
		if i == len(workers)-1 {
			endY = request.EndY // 最后一个 worker 处理到最后
		}

		// 创建独立的 CalRequest 对象
		workerReq := copyCalRequest(request, startY, endY)

		go func(workerAddress string, req stdstruct.CalRequest, idx int) {
			defer wg.Done()
			fmt.Printf("Attempting to connect to worker: %s with StartY=%d, EndY=%d\n", workerAddress, req.StartY, req.EndY)
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

			select {
			case responseChannels[idx] <- *response:
				// Successfully written to channel
			case <-time.After(2 * time.Second):
				fmt.Printf("Timeout while waiting to write to response channel for worker %s\n", workerAddress)
			}
		}(workerAddress, workerReq, i)
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
			retries := 3
			for r := 0; r < retries; r++ {
				select {
				case result := <-responseChannel[i]:
					mu.Lock() // Lock to safely append to the result slice
					res = append(res, result)
					mu.Unlock()
					fmt.Printf("Received response from worker %d for topic: %s\n", i+1, topic)
					return
				case <-time.After(5 * time.Second):
					fmt.Printf("Timeout waiting for response from worker %d, retrying...\n", i+1)
				}
			}
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

	for _, workerRes := range result {

		fmt.Printf("Merging section StartY=%d, EndY=%d, StartX=%d, EndX=%d\n",
			workerRes.StartY, workerRes.EndY, workerRes.StartX, workerRes.EndX)

		for y := 0; y < workerRes.EndY-workerRes.StartY; y++ {
			for x := 0; x < workerRes.EndX-workerRes.StartX; x++ {
				finalWorld[workerRes.StartY+y][workerRes.StartX+x] = workerRes.World[y][x]
			}
		}
		finalAliveCells = append(finalAliveCells, workerRes.AliveCells...)
	}
	res.World = finalWorld
	res.AliveCells = finalAliveCells
	res.Results = result

	fmt.Printf("Completed collecting responses for topic: %s\n", req.Topic)
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
