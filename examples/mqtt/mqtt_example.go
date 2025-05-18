// Package main provides an example of using the MQTT transport for gomcp
package main

import (
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/localrivet/gomcp/client"
	"github.com/localrivet/gomcp/server"
	"github.com/localrivet/gomcp/transport/mqtt"
)

func main() {
	// Create a channel to listen for termination signals
	signals := make(chan os.Signal, 1)
	signal.Notify(signals, syscall.SIGINT, syscall.SIGTERM)

	// This example assumes you have a local MQTT broker running
	// such as Mosquitto on the default port
	brokerURL := "tcp://localhost:1883"

	// Start the server in a goroutine
	startServer(brokerURL)

	// Wait a bit for the server to initialize
	time.Sleep(1 * time.Second)

	// Start the client
	go runClient(brokerURL)

	// Wait for termination signal
	<-signals
	fmt.Println("\nShutdown signal received, exiting...")
}

func startServer(brokerURL string) {
	// Create a new server
	srv := server.NewServer("mqtt-example-server")

	// Configure the server with MQTT transport
	srv.AsMQTT(brokerURL,
		mqtt.WithClientID("mcp-example-server"),
		mqtt.WithQoS(1),
		mqtt.WithTopicPrefix("mcp-example"),
	)

	// Register a simple echo tool
	srv.Tool("echo", "Echo the message back", func(ctx *server.Context, args struct {
		Message string `json:"message"`
	}) (map[string]interface{}, error) {
		fmt.Printf("Server received: %s\n", args.Message)
		return map[string]interface{}{
			"message": args.Message,
		}, nil
	})

	// Start the server in a goroutine
	go func() {
		fmt.Println("Starting MQTT server on", brokerURL)
		if err := srv.Run(); err != nil {
			log.Fatalf("Server error: %v", err)
		}
	}()
}

func runClient(brokerURL string) {
	// Create a new client with MQTT transport
	c, err := client.NewClient("mqtt-example-client",
		client.WithMQTT(brokerURL,
			client.WithMQTTClientID("mcp-example-client"),
			client.WithMQTTQoS(1),
			client.WithMQTTTopicPrefix("mcp-example"),
		),
		client.WithConnectionTimeout(5*time.Second),
		client.WithRequestTimeout(30*time.Second),
	)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}
	defer c.Close()

	// Call the echo tool
	echoResult, err := c.CallTool("echo", map[string]interface{}{
		"message": "Hello from MQTT client!",
	})
	if err != nil {
		log.Fatalf("Echo call failed: %v", err)
	}
	fmt.Printf("Echo result: %v\n", echoResult)

	// Wait a moment to allow printing of results
	time.Sleep(500 * time.Millisecond)
}
