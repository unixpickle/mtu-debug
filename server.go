package main

import (
	"flag"
	"log"
	"net"

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
	addr := net.UDPAddr{
		Port: s.Port,
		IP:   net.IPv4zero,
	}
	conn, err := net.ListenUDP("udp", &addr)
	essentials.Must(err)
	defer conn.Close()

	for {
		data := make([]byte, 16384)
		oob := make([]byte, 16384)
		n, _, _, addr, err := conn.ReadMsgUDP(data, oob)
		essentials.Must(err)
		if n < 2 {
			log.Printf("%s: received %d bytes, expected at least 2", addr, n)
			continue
		}
		respSize := (int(data[0]) << 8) | int(data[1])
		resp := make([]byte, respSize)
		for i := range resp {
			resp[i] = data[i%n]
		}
		log.Printf("%s: responding to %d bytes with %d bytes", addr, n, len(resp))
		n, _, err = conn.WriteMsgUDP(resp, nil, addr)
		if err != nil {
			log.Printf("%s: failed to send data: %s", addr, err)
		}
		log.Printf("%s: actually sent %d bytes", addr, n)
	}
}
