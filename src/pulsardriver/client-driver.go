package pulsardriver

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/apache/pulsar/pulsar-client-go/pulsar"
	"github.com/pulsar-beam/src/db"
)

// TopicStatus -
type TopicStatus int

const (
	activated TopicStatus = iota
	pending
	deactivated
	suspended
)

// TopicConfig -
type TopicConfig struct {
	TopicFN   string
	Token     string
	Tenant    string
	Status    TopicStatus
	CreatedAt time.Time
	UpdatedAt time.Time
}

// TopicConfig2 is a simplified configuration for prototype.
// Assumptions are made with no token verification for subsquent connections
// No status support.
type TopicConfig2 struct {
	TopicFN   string
	Token     string
	PulsarURL string
}

var connections = make(map[string]pulsar.Client)
var producers = make(map[string]pulsar.Producer)
var consumers = make(map[string]pulsar.Consumer)

var trustStore string

func init() {
	// TODO: add code to tell CentOS or Ubuntu
	trustStore = "/etc/ssl/certs/ca-bundle.crt"
}

func getTopicDriver(url, tokenStr string) pulsar.Client {
	key := tokenStr
	token := pulsar.NewAuthenticationToken(tokenStr)
	driver, ok := connections[key]
	if ok {
		return driver
	}

	var err error
	driver, err = pulsar.NewClient(pulsar.ClientOptions{
		URL:                     url,
		Authentication:          token,
		TLSTrustCertsFilePath:   trustStore,
		IOThreads:               3,
		OperationTimeoutSeconds: 5,
	})

	if err != nil {
		log.Fatalf("Could not instantiate Pulsar client: %v", err)
		return nil
	}

	connections[key] = driver

	return driver
}

func getProducer(url, token, topic string) pulsar.Producer {
	key := topic
	p, ok := producers[key]
	if ok {
		return p
	}

	driver := getTopicDriver(url, token)
	if driver == nil {
		return nil
	}

	var err error
	p, err = driver.CreateProducer(pulsar.ProducerOptions{
		Topic: topic,
	})

	if err != nil {
		return nil
	}

	producers[key] = p
	return p
}

// SendToPulsar sends data to a Pulsar producer.
func SendToPulsar(url, token, topic string, data []byte) error {
	p := getProducer(url, token, topic)
	if p == nil {
		return errors.New("Failed to create Pulsar producer")
	}

	ctx := context.Background()

	// Create a different message to send asynchronously
	asyncMsg := pulsar.ProducerMessage{
		Payload: data,
	}

	p.SendAsync(ctx, asyncMsg, func(msg pulsar.ProducerMessage, err error) {
		if err != nil {
			log.Fatal(err)
			// TODO: add retry
		}

	})
	return nil
}

// GetConsumer gets the matching Pulsar consumer
func GetConsumer(url, token, topic string) pulsar.Consumer {
	key, _ := db.GetKeyFromNames(topic, url)
	/*if err != nil {
		return nil, err
	}*/
	consumer := consumers[key]
	if consumer != nil {
		return consumer
	}

	driver := getTopicDriver(url, token)
	if driver == nil {
		return nil
	}

	var err error
	consumer, err = driver.Subscribe(pulsar.ConsumerOptions{
		Topic:            topic,
		SubscriptionName: "my-subscription",
	})
	if err != nil {
		log.Fatal(err)
		return nil
	}
	consumers[key] = consumer
	return consumer
}

// CancelConsumer closes consumer
func CancelConsumer(key string) {
	c, ok := consumers[key]
	if ok {
		err := c.Close()
		delete(consumers, key)
		if err != nil {
			log.Printf("cancel consumer failed %v", err.Error())
		}
		log.Println("consumer  close() called")
	} else {
		log.Printf("failed to locate consumer key %v", key)
	}
}
