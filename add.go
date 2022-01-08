package main

import "log"

var addChan = make(chan string)
var addProjectConsumer AddProjectConsumer

type AddProjectConsumer struct{}

func (consumer *AddProjectConsumer) Consume(url string) {
	// perform task
	log.Printf("performing task %s", url)
}

func addWorker() {
	for i := range addChan {
		addProjectConsumer.Consume(i)
	}
}
