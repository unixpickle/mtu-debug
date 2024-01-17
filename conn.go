package main

import (
	"errors"
	"fmt"
	"log"
	"math/rand"
	"net"
	"os"
	"syscall"
	"time"

	"github.com/unixpickle/essentials"
	"golang.org/x/net/ipv4"
)

const (
	icmpProtocolNumber = 1
	udpProtocolNumber  = 17
)

var ErrTimeout = errors.New("read operation timed out")

type incomingIP struct {
	Data []byte
	Addr *net.IPAddr
}

type incomingUDP struct {
	Data []byte
	Addr *net.UDPAddr
}

type PacketConn struct {
	rawUDPConn  *os.File
	rawICMPConn *os.File
	udpConn     *net.UDPConn
	ipChan      chan incomingIP
	udpChan     chan incomingUDP
	closeChan   chan struct{}
	remote      *net.UDPAddr
}

func NewPacketConnClient(dstHost net.IP, dstPort int) (*PacketConn, error) {
	if dstHost.To4() == nil {
		return nil, errors.New("packet conn only supports IPv4")
	}
	addr := &net.UDPAddr{
		Port: dstPort,
		IP:   dstHost,
	}
	udpConn, err := net.DialUDP("udp4", nil, addr)
	essentials.Must(err)

	udpFile, err := CreateSocket(syscall.IPPROTO_UDP)
	icmpFile, err := CreateSocket(syscall.IPPROTO_ICMP)

	p := &PacketConn{
		rawUDPConn:  udpFile,
		rawICMPConn: icmpFile,
		udpConn:     udpConn,
		ipChan:      make(chan incomingIP, 1),
		udpChan:     make(chan incomingUDP, 1),
		closeChan:   make(chan struct{}, 1),
		remote:      addr,
	}
	go p.udpReadLoop()
	go p.icmpReadLoop()
	return p, nil
}

func NewPacketConnServer(port int) (*PacketConn, error) {
	addr := net.UDPAddr{
		Port: port,
		IP:   net.IPv4zero,
	}
	udpConn, err := net.ListenUDP("udp", &addr)
	if err != nil {
		return nil, err
	}

	udpFile, err := CreateSocket(syscall.IPPROTO_UDP)
	icmpFile, err := CreateSocket(syscall.IPPROTO_ICMP)

	p := &PacketConn{
		rawUDPConn:  udpFile,
		rawICMPConn: icmpFile,
		udpConn:     udpConn,
		ipChan:      make(chan incomingIP, 1),
		udpChan:     make(chan incomingUDP, 1),
		closeChan:   make(chan struct{}, 1),
	}
	go p.udpReadLoop()
	go p.icmpReadLoop()
	return p, nil
}

func CreateSocket(proto int) (*os.File, error) {
	socket, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_RAW, proto)
	if err != nil {
		return nil, err
	}
	syscall.SetsockoptByte(socket, syscall.IPPROTO_IP, syscall.IP_HDRINCL, 1)

	// This option makes the kernel allow us to send packets larger than
	// the path MTU it has previously discovered.
	// This allows us to keep sending packets after a packet too large
	// ICMP message has come in.
	if err := syscall.SetsockoptByte(socket, syscall.IPPROTO_IP, syscall.IP_MTU_DISCOVER,
		syscall.IP_PMTUDISC_PROBE); err != nil {
		return nil, err
	}

	ipFile := os.NewFile(uintptr(socket), fmt.Sprintf("IP socket (fd %d)", socket))
	return ipFile, nil
}

func (p *PacketConn) LocalAddr() net.Addr {
	return p.udpConn.LocalAddr()
}

// Send writes a UDP packet to the destination.
// For clients, addr should be nil.
func (p *PacketConn) Send(data []byte, addr *net.UDPAddr) error {
	if len(data) > 0xffff-(20+8) {
		return errors.New("cannot encode packet this large")
	}
	if addr == nil {
		addr = p.udpConn.RemoteAddr().(*net.UDPAddr)
	}
	srcAddr := p.udpConn.LocalAddr().(*net.UDPAddr)

	ipHeader := ipv4.Header{
		Version:  4,
		Len:      20,
		TotalLen: 20 + 8 + len(data),
		ID:       rand.Intn(0x10000),
		Flags:    ipv4.DontFragment,
		TTL:      32,
		Protocol: udpProtocolNumber,
		Src:      srcAddr.IP,
		Dst:      addr.IP,
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
	udpHeader[0] = byte(srcAddr.Port >> 8)
	udpHeader[1] = byte(srcAddr.Port & 0xff)
	udpHeader[2] = byte(addr.Port >> 8)
	udpHeader[3] = byte(addr.Port & 0xff)
	udpHeader[4] = byte((len(data) + 8) >> 8)
	udpHeader[5] = byte((len(data) + 8) & 0xff)

	sendData := append(append(encoded, udpHeader...), data...)

	var sockaddr syscall.SockaddrInet4
	sockaddr.Port = addr.Port
	copy(sockaddr.Addr[:], addr.IP.To4())
	if err := syscall.Sendto(int(p.rawUDPConn.Fd()), sendData, 0, &sockaddr); err != nil {
		return fmt.Errorf("failed to send UDP packet: %s", err)
	}

	return nil
}

func ipv4Checksum(data []byte) uint16 {
	var sum uint32
	for i := 0; i < len(data); i += 2 {
		sum += (uint32(data[i]) << 8) | uint32(data[i])
	}
	for sum > 0xffff {
		sum = (sum & 0xffff) + (sum >> 16)
	}
	return ^uint16(sum)
}

// Recv reads either a UDP packet or an ICMP packet.
func (p *PacketConn) Recv(timeout time.Duration) ([]byte, *net.UDPAddr, *net.IPAddr, error) {
	select {
	case packet := <-p.ipChan:
		return packet.Data, nil, packet.Addr, nil
	case packet := <-p.udpChan:
		return packet.Data, packet.Addr, nil, nil
	case <-time.After(timeout):
		return nil, nil, nil, ErrTimeout
	}
}

func (p *PacketConn) Close() error {
	select {
	case <-p.closeChan:
		return errors.New("already closed")
	default:
	}

	// Tell background Goroutines not to panic.
	close(p.closeChan)

	p.rawUDPConn.Close()
	p.rawICMPConn.Close()
	p.udpConn.Close()
	return nil
}

func (p *PacketConn) udpReadLoop() {
	localAddr := p.udpConn.LocalAddr().(*net.UDPAddr)
	for {
		packet := make([]byte, 32768)
		n, _, err := syscall.Recvfrom(int(p.rawUDPConn.Fd()), packet, 0)
		if err != nil {
			select {
			case <-p.closeChan:
				return
			default:
			}
			log.Fatalf("error reading from raw UDP connection: %s", err)
		}
		header, err := ipv4.ParseHeader(packet[:n])
		if err != nil {
			log.Printf("received invalid IPv4 header: %s", err)
			continue
		}
		if header.Protocol != udpProtocolNumber {
			continue
		}
		if p.remote != nil && !header.Dst.Equal(localAddr.IP) {
			// This path only makes sense if we are connected to a remote
			// address. If we are a server, we don't know which address we
			// will receive messages on.
			continue
		}
		if header.Len+8 >= n {
			continue
		}
		udpHeader := packet[header.Len:][:8]
		srcPort := (uint16(udpHeader[0]) << 8) | uint16(udpHeader[1])
		dstPort := (uint16(udpHeader[2]) << 8) | uint16(udpHeader[3])
		length := (uint16(udpHeader[4]) << 8) | uint16(udpHeader[5])
		if int(dstPort) != localAddr.Port {
			continue
		}
		if int(length)+header.Len > n || length < 8 {
			log.Printf("UDP length overflow detected from %s", header.Src)
			continue
		}
		if p.remote != nil {
			if !header.Src.Equal(p.remote.IP) || int(srcPort) != p.remote.Port {
				continue
			}
		}
		body := packet[header.Len+8:][:length-8]
		msg := incomingUDP{
			Data: body,
			Addr: &net.UDPAddr{
				IP:   header.Src,
				Port: int(srcPort),
			},
		}
		select {
		case p.udpChan <- msg:
		case <-p.closeChan:
			return
		}
	}
}

func (p *PacketConn) icmpReadLoop() {
	localAddr := p.udpConn.LocalAddr().(*net.UDPAddr)
	for {
		packet := make([]byte, 32768)
		n, _, err := syscall.Recvfrom(int(p.rawICMPConn.Fd()), packet, 0)
		if err != nil {
			select {
			case <-p.closeChan:
				return
			default:
			}
			log.Fatalf("error reading from raw UDP connection: %s", err)
		}
		header, err := ipv4.ParseHeader(packet[:n])
		if err != nil {
			log.Printf("received invalid IPv4 header: %s", err)
			continue
		}
		if header.Protocol != icmpProtocolNumber {
			continue
		}
		if !header.Dst.Equal(localAddr.IP) {
			continue
		}
		body := packet[header.Len:header.TotalLen]
		msg := incomingIP{
			Data: body,
			Addr: &net.IPAddr{
				IP: header.Src,
			},
		}
		select {
		case p.ipChan <- msg:
		case <-p.closeChan:
			return
		}
	}
}
