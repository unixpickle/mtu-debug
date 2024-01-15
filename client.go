package main

import (
	"flag"
	"log"
	"math/rand"
	"net"
	"time"

	"github.com/unixpickle/essentials"
)

type Client struct {
	Host string
	Port int

	RequestSize  int
	ResponseSize int
}

func (c *Client) FlagSet() *flag.FlagSet {
	clientCmd := flag.NewFlagSet("client", flag.ExitOnError)
	clientCmd.StringVar(&c.Host, "host", "", "host of remote server")
	clientCmd.IntVar(&c.Port, "port", 1337, "port of remote server")
	clientCmd.IntVar(&c.RequestSize, "request-size", 1500, "size of request")
	clientCmd.IntVar(&c.ResponseSize, "response-size", 1500, "desired size of response")
	return clientCmd
}

func (c *Client) Run() {
	if c.RequestSize < 2 {
		essentials.Die("request size must be at least 2 bytes")
	}
	if c.ResponseSize < 1 {
		essentials.Die("response must be at least 1 byte")
	}

	conn, err := NewPacketConnClient(net.ParseIP(c.Host), c.Port)
	essentials.Must(err)
	defer conn.Close()

	data := make([]byte, c.RequestSize)
	data[0] = byte(c.ResponseSize >> 8)
	data[1] = byte(c.ResponseSize)
	rand.Read(data[2:])

RequestLoop:
	for i := 0; i < 10; i++ {
		if i > 0 {
			time.Sleep(time.Second)
		}
		log.Printf("sending request from %s => %s:%d ...", conn.LocalAddr(), c.Host, c.Port)

		err := conn.Send(data, nil)
		essentials.Must(err)

		for {
			payload, udpAddr, ipAddr, err := conn.Recv(time.Second)
			if err != nil {
				log.Printf("retrying after error: %s", err)
				continue RequestLoop
			} else if udpAddr != nil {
				if len(payload) != c.ResponseSize {
					log.Printf("expected size %d but got %d; retrying...",
						c.ResponseSize, len(payload))
					continue RequestLoop
				}
				for i := 0; i < c.ResponseSize; i++ {
					if payload[i] != data[i%c.RequestSize] {
						log.Printf("invalid bytes received; retrying...")
						continue RequestLoop
					}
				}
				log.Println("SUCCESS: received correct response!")
				return
			} else {
				LogICMPMessage(ipAddr, payload)
			}
		}
	}
	log.Println("FAIL: giving up after too many attempts!")
}
