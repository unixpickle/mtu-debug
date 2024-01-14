package main

import (
	"errors"
	"flag"
	"log"
	"math/rand"
	"net"
	"os"
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
	addr := &net.UDPAddr{
		Port: c.Port,
		IP:   net.ParseIP(c.Host),
	}
	conn, err := net.DialUDP("udp", nil, addr)
	essentials.Must(err)
	defer conn.Close()

	go LogICMPMessages(conn.LocalAddr().(*net.UDPAddr))

	data := make([]byte, c.RequestSize)
	data[0] = byte(c.ResponseSize >> 8)
	data[1] = byte(c.ResponseSize)
	rand.Read(data[2:])

RequestLoop:
	for i := 0; i < 10; i++ {
		if i > 0 {
			time.Sleep(time.Second)
		}
		log.Printf("sending request from %s => %s ...", conn.LocalAddr(), addr)

		n, err := conn.Write(data)
		essentials.Must(err)
		if n < c.RequestSize {
			log.Fatalf("attempted to write %d bytes but wrote %d", c.RequestSize, n)
		}

		conn.SetReadDeadline(time.Now().Add(time.Second * 5))
		payload := make([]byte, c.ResponseSize)
		n, _, _, _, err = conn.ReadMsgUDP(payload, nil)
		if errors.Is(err, os.ErrDeadlineExceeded) {
			log.Printf("timeout waiting for response; retrying...")
			continue
		}

		essentials.Must(err)
		if n != c.ResponseSize {
			log.Printf("expected size %d but got %d; retrying...", c.ResponseSize, n)
			continue
		}

		for i := 0; i < c.ResponseSize; i++ {
			if payload[i] != data[i%c.RequestSize] {
				log.Printf("invalid bytes received; retrying...")
				continue RequestLoop
			}
		}
		log.Println("SUCCESS: received correct response!")
		return
	}
	log.Println("FAIL: giving up after too many attempts!")
}
