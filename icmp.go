package main

import (
	"log"
	"net"

	"golang.org/x/net/icmp"
)

const (
	ICMPv4 = 1
	ICMPv6 = 58
)

func LogICMPMessages(addr *net.UDPAddr) {
	protoStr := "ip4:icmp"
	proto := ICMPv4
	if addr.IP.To4() == nil {
		protoStr = "ip6:ipv6-icmp"
		proto = ICMPv6
	}
	packetConn, err := icmp.ListenPacket(protoStr, addr.IP.String())
	if err != nil {
		log.Printf("WARNING: cannot listen for ICMP packets: %s", err)
		return
	}
	defer packetConn.Close()
	for {
		packet := make([]byte, 32768)
		size, addr, err := packetConn.ReadFrom(packet)
		if err != nil {
			log.Printf("WARNING: error reading ICMP traffic: %s", err)
			return
		}
		packet = packet[:size]
		msg, err := icmp.ParseMessage(proto, packet)
		if err != nil {
			log.Printf("ICMP from %s: failed to parse: %s", addr, err)
			continue
		}
		if ptb, ok := msg.Body.(*icmp.PacketTooBig); ok {
			log.Printf("ICMP from %s: packet too big (MTU=%d)", addr, ptb.MTU)
		}
	}
}
