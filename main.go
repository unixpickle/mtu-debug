package main

import (
	"fmt"
	"os"

	"github.com/unixpickle/essentials"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Usage: mtu_debug <subcommand> [args...]")
		fmt.Fprintln(os.Stderr, "")
		fmt.Fprintln(os.Stderr, "Available sub-commands:")
		fmt.Fprintln(os.Stderr, "    server      listen for incoming requests")
		fmt.Fprintln(os.Stderr, "    client      send a request")
		fmt.Fprintln(os.Stderr, "    needtofrag  send an ICMP need to fragment packet")
		os.Exit(1)
	}

	switch os.Args[1] {
	case "client":
		client := &Client{}
		client.FlagSet().Parse(os.Args[2:])
		client.Run()
	case "server":
		server := &Server{}
		server.FlagSet().Parse(os.Args[2:])
		server.Run()
	case "needtofrag":
		ntf := &NeedToFrag{}
		ntf.FlagSet().Parse(os.Args[2:])
		ntf.Run()
	default:
		essentials.Die("unknown sub-command: " + os.Args[1])
	}
}
