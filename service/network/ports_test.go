package network

import (
	"net"
	"testing"
	"time"

	"github.com/safing/portmaster/service/network/packet"
)

// TestPortIsBindable checks that ports which cannot be bound are recognized as
// such. Parts of the dynamic port range may be reserved by the OS or already
// in use, which makes binding fail even though the port looks unused.
func TestPortIsBindable(t *testing.T) {
	t.Parallel()

	udp := uint8(packet.UDP)

	// Occupy a port.
	udpConn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		t.Fatalf("failed to occupy udp port: %s", err)
	}
	defer func() { _ = udpConn.Close() }()
	udpPort := portOf(t, udpConn.LocalAddr())

	if portIsBindable(udp, udpPort) {
		t.Errorf("udp port %d is in use, but was reported as bindable", udpPort)
	}

	// A port that was just released must be bindable again.
	freeConn, err := net.ListenPacket("udp", ":0")
	if err != nil {
		t.Fatalf("failed to find a free udp port: %s", err)
	}
	freePort := portOf(t, freeConn.LocalAddr())
	_ = freeConn.Close()

	if !portIsBindable(udp, freePort) {
		t.Errorf("udp port %d is free, but was reported as not bindable", freePort)
	}
}

// TestUnusablePorts checks that ports which failed to bind are remembered, so
// that they are not handed out again.
func TestUnusablePorts(t *testing.T) {
	t.Parallel()

	udp := uint8(packet.UDP)
	const port = 65123

	forgetPort(t, udp, port)
	if portIsKnownUnusable(udp, port) {
		t.Fatalf("udp port %d is unexpectedly known as unusable", port)
	}

	markPortUnusable(udp, port)

	if !portIsKnownUnusable(udp, port) {
		t.Errorf("udp port %d was marked unusable, but is not remembered", port)
	}
	if portIsKnownUnusable(uint8(packet.TCP), port) {
		t.Errorf("tcp port %d was marked unusable for udp only", port)
	}
}

// TestPortIsBindableSkipsTCP checks that TCP ports are not verified. Checking
// them would mean listening on the port, which is not what it is used for, so
// an unbindable TCP port is left to the caller to handle.
func TestPortIsBindableSkipsTCP(t *testing.T) {
	t.Parallel()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("failed to occupy tcp port: %s", err)
	}
	defer func() { _ = listener.Close() }()
	port := portOf(t, listener.Addr())

	if !portIsBindable(uint8(packet.TCP), port) {
		t.Errorf("tcp port %d should not be checked", port)
	}
}

// TestUnusablePortsExpire checks that a port is tried again after its entry
// expired. A port may be unbindable only because something else held it for a
// moment, so entries must not be remembered forever.
func TestUnusablePortsExpire(t *testing.T) {
	t.Parallel()

	udp := uint8(packet.UDP)
	const port = 65432

	forgetPort(t, udp, port)
	markPortUnusable(udp, port)

	// Expire the entry.
	unusablePortsLock.Lock()
	unusablePorts[unusablePortKey(udp, port)] = time.Now().Add(-time.Second)
	unusablePortsLock.Unlock()

	if portIsKnownUnusable(udp, port) {
		t.Errorf("udp port %d is still remembered after its entry expired", port)
	}
}

// portOf returns the port of the given address.
func portOf(t *testing.T, addr net.Addr) uint16 {
	t.Helper()

	var port int
	switch a := addr.(type) {
	case *net.UDPAddr:
		port = a.Port
	case *net.TCPAddr:
		port = a.Port
	default:
		t.Fatalf("unexpected address type %T", addr)
	}

	return uint16(port) //nolint:gosec // Port is within uint16.
}

// forgetPort removes the given port from the unusable ports, now and when the
// test finishes, so that tests do not affect each other.
func forgetPort(t *testing.T, protocol uint8, port uint16) {
	t.Helper()

	forget := func() {
		unusablePortsLock.Lock()
		defer unusablePortsLock.Unlock()

		delete(unusablePorts, unusablePortKey(protocol, port))
	}

	forget()
	t.Cleanup(forget)
}
