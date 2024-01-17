package main

import (
	"flag"
	"log"
	"math/rand"
	"net"
	"syscall"

	"github.com/unixpickle/essentials"
	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

type NeedToFrag struct {
	SrcHost string
	DstHost string
	MTU     int
	TTL     int
}

func (n *NeedToFrag) FlagSet() *flag.FlagSet {
	clientCmd := flag.NewFlagSet("needtofrag", flag.ExitOnError)
	clientCmd.StringVar(&n.SrcHost, "src-host", "", "source address to include in the packet")
	clientCmd.StringVar(&n.DstHost, "dst-host", "", "host address to send to")
	clientCmd.IntVar(&n.MTU, "mtu", 1420, "MTU to send in packet")
	clientCmd.IntVar(&n.TTL, "ttl", 64, "MTU to send in packet")
	return clientCmd
}

func (n *NeedToFrag) Run() {
	connSocket, err := CreateSocket(syscall.IPPROTO_RAW)
	essentials.Must(err)
	defer connSocket.Close()

	data := n.EncodePacket()
	var sockaddr syscall.SockaddrInet4
	copy(sockaddr.Addr[:], net.ParseIP(n.DstHost).To4())
	if err := syscall.Sendto(int(connSocket.Fd()), data, 0, &sockaddr); err != nil {
		log.Fatalf("failed to send ICMP packet: %s", err)
	} else {
		log.Printf("sent ICMP packet")
	}
}

func (n *NeedToFrag) EncodePacket() []byte {
	origPacket := n.DummySourcePacket()

	msg := icmp.Message{
		Type: ipv4.ICMPType(3),
		Code: 4,
		Body: &icmp.DstUnreach{
			Data: origPacket[:556],
		},
	}
	icmpEnc, err := msg.Marshal(nil)

	// This changes the checksum, which we don't recompute at the moment.
	// icmpEnc[6] = uint8(n.MTU >> 8)
	// icmpEnc[7] = uint8(n.MTU)

	ipHeader := ipv4.Header{
		Version:  4,
		Len:      20,
		TotalLen: 20 + len(icmpEnc),
		ID:       rand.Intn(0x10000),
		Flags:    ipv4.DontFragment,
		TTL:      n.TTL,
		Protocol: icmpProtocolNumber,
		Src:      net.ParseIP(n.SrcHost),
		Dst:      net.ParseIP(n.DstHost),
	}
	encodedRaw, err := ipHeader.Marshal()
	if err != nil {
		panic(err)
	}
	checksum := ipv4Checksum(encodedRaw)
	ipHeader.Checksum = int(checksum)
	encoded, err := ipHeader.Marshal()
	if err != nil {
		panic(err)
	}
	return append(encoded, icmpEnc...)
}

func (n *NeedToFrag) DummySourcePacket() []byte {
	srcAddr := net.ParseIP(n.DstHost)
	dstAddr := net.ParseIP(n.SrcHost)
	port := 1337

	data := make([]byte, 1472)

	ipHeader := ipv4.Header{
		Version:  4,
		Len:      20,
		TotalLen: 20 + 8 + len(data),
		ID:       rand.Intn(0x10000),
		Flags:    ipv4.DontFragment,
		TTL:      32,
		Protocol: udpProtocolNumber,
		Src:      srcAddr,
		Dst:      dstAddr,
	}
	encodedRaw, err := ipHeader.Marshal()
	if err != nil {
		panic(err)
	}
	checksum := ipv4Checksum(encodedRaw)
	ipHeader.Checksum = int(checksum)
	encoded, err := ipHeader.Marshal()
	if err != nil {
		panic(err)
	}

	udpHeader := make([]byte, 8)
	udpHeader[0] = byte(port >> 8)
	udpHeader[1] = byte(port & 0xff)
	udpHeader[2] = byte(port >> 8)
	udpHeader[3] = byte(port & 0xff)
	udpHeader[4] = byte((len(data) + 8) >> 8)
	udpHeader[5] = byte((len(data) + 8) & 0xff)

	return append(append(encoded, udpHeader...), data...)
}
