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
	responseCh = make(map[string]chan stdstruct.CalResponse)
	workers    = []string{"52.54.105.119:8031", "3.93.79.176:8032"}
	topicmx    sync.RWMutex
)

// Create a new topic as a buffered channel.
func newTopic(topic string, buflen int) {
	topicmx.Lock()
	defer topicmx.Unlock()
	if _, ok := topics[topic]; !ok {
		fmt.Printf("Creating new topic: %s with buffer length: %d\n", topic, buflen)
		topics[topic] = make(chan stdstruct.CalRequest, buflen)
		responseCh[topic] = make(chan stdstruct.CalResponse, buflen)
	}
}

// add task to the specific Topic task queue
func publish(topic string, request stdstruct.CalRequest) (err error) {
	topicmx.RLock()
	defer topicmx.RUnlock()
	if ch, ok := topics[topic]; ok {
		fmt.Printf("Publishing request to topic: %s\n", topic)
		ch <- request

		var wg sync.WaitGroup
		wg.Add(len(workers))

		for _, workerAddress := range workers {
			go func(workerAddress string, req stdstruct.CalRequest) {
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
				topicmx.RLock()
				responseChannel, exists := responseCh[topic]
				topicmx.RUnlock()
				if exists {
					fmt.Printf("Publishing response from worker %s to response channel for topic: %s\n", workerAddress, topic)
					responseChannel <- *response
				}
			}(workerAddress, request)
		}
		wg.Wait()
	} else {
		fmt.Printf("Topic %s not found\n", topic)
	}
	return
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

	for i := 0; i < expectedResponses; i++ {
		result := <-responseChannel
		fmt.Printf("Received response for topic: %s\n", topic)
		res = append(res, result)
	}
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
