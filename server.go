package main

import (
	"errors"
	"flag"
	"log"
	"time"

	"github.com/unixpickle/essentials"
)

type Server struct {
	Port int
}

func (s *Server) FlagSet() *flag.FlagSet {
	serverCmd := flag.NewFlagSet("server", flag.ExitOnError)
	serverCmd.IntVar(&s.Port, "port", 1337, "port for server")
	return serverCmd
}

func (s *Server) Run() {
	conn, err := NewPacketConnServer(s.Port)
	essentials.Must(err)
	defer conn.Close()

	for {
		data, udpAddr, ipAddr, err := conn.Recv(time.Minute)
		if errors.Is(err, ErrTimeout) {
			continue
		}
		essentials.Must(err)
		if udpAddr != nil {
			if len(data) < 2 {
				log.Printf("%s: received %d bytes, expected at least 2", udpAddr, len(data))
				continue
			}
			respSize := (int(data[0]) << 8) | int(data[1])
			resp := make([]byte, respSize)
			for i := range resp {
				resp[i] = data[i%len(data)]
			}
			log.Printf("%s: responding to %d bytes with %d bytes", udpAddr, len(data), len(resp))
			err = conn.Send(resp, udpAddr)
			if err != nil {
				log.Printf("%s: failed to send data: %s", udpAddr, err)
			}
		} else {
			LogICMPMessage(ipAddr, data)
		}
	}
}
