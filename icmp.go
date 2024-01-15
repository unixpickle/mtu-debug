package main

import (
	"log"
	"net"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

func LogICMPMessage(src *net.IPAddr, data []byte) {
	icmpMessage, err := icmp.ParseMessage(icmpProtocolNumber, data)
	if err != nil {
		log.Printf("%s: invalid ICMP message: %s", src, err)
		return
	}
	if msg, ok := icmpMessage.Body.(*icmp.DstUnreach); ok {
		origHeader, err := ipv4.ParseHeader(msg.Data)
		if err != nil {
			log.Printf("%s: invalid IP header inside ICMP unreachable message: %s", src, err)
			return
		}
		if icmpMessage.Code == 4 {
			nextMtu := (int(data[6]) << 8) | int(data[7])
			log.Printf("%s: ICMP: destination (%s) unreachable; fragmentation needed (mtu=%d)",
				src, origHeader.Dst, nextMtu)
		}
	} else {
		log.Printf("%s: ICMP message: type=%d, code=%d, size=%d",
			src, icmpMessage.Type, icmpMessage.Code, len(data))
	}
}
