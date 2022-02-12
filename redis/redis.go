package redis

import (
	"fmt"
	"github.com/go-redis/redis/v7"
)

var client *redis.Client
var subscription <-chan *redis.Message

const channel = "cfupdates"

var counter = 0

func init() {
	client = redis.NewClient(&redis.Options{
		Addr:     "localhost:6379",
		Password: "", // no password set
		DB:       0,  // use default DB
	})

	pong, err := client.Ping().Result()
	fmt.Println(pong, err)

	publish := client.Subscribe(channel)

	// Wait for confirmation that subscription is created before publishing anything.
	_, err = publish.Receive()
	if err != nil {
		panic(err)
	}

	subscription = publish.Channel()
	go tick()
}

func Submit(slug string) error{
	return client.Publish(channel, slug).Err()
}

func tick() {
	for msg := range subscription {
		syncProject(msg.Payload)
	}
}

func syncProject(slug string) {
	fmt.Printf("Syncing [%d] %s\n", counter, slug)
	counter++
}
