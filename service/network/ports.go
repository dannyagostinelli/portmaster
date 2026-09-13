package network

import (
	"fmt"
	"net"
	"sync"
	"time"

	"github.com/safing/portmaster/base/log"
	"github.com/safing/portmaster/base/rng"
	"github.com/safing/portmaster/service/network/packet"
)

// Ports that failed to bind are skipped until their entry expires.
const unusablePortTTL = 1 * time.Hour

var (
	unusablePorts     = make(map[uint32]time.Time)
	unusablePortsLock sync.Mutex
)

// GetUnusedLocalPort returns a local port of the specified protocol that is
// currently unused and is unlikely to be used within the next seconds.
func GetUnusedLocalPort(protocol uint8) (port uint16, ok bool) {
	allConns := conns.clone()
	tries := 1000

	// Try up to 1000 times to find an unused port.
nextPort:
	for i := range tries {
		// Generate random port between 10000 and 65535
		rN, err := rng.Number(55535)
		if err != nil {
			log.Warningf("network: failed to generate random port: %s", err)
			return 0, false
		}
		port := uint16(rN + 10000)

		// Shrink range when we chew through the tries.
		portRangeStart := port - 10

		// Check if the generated port is unused.
	nextConnection:
		for _, conn := range allConns {
			switch {
			case !conn.DataIsComplete():
				// Skip connection if the data is not complete.
				continue nextConnection

			case conn.Entity.Protocol != protocol:
				// Skip connection if the protocol does not match the protocol of interest.
				continue nextConnection

			case conn.LocalPort <= port && conn.LocalPort >= portRangeStart:
				// Skip port if the local port is in dangerous proximity.
				// Consecutive port numbers are very common.
				continue nextPort
			}
		}

		// Skip ports that failed to bind before.
		if portIsKnownUnusable(protocol, port) {
			continue nextPort
		}

		// Make sure the port can actually be bound.
		if !portIsBindable(protocol, port) {
			markPortUnusable(protocol, port)
			continue nextPort
		}

		// Log if it took more than 10 attempts.
		if i >= 10 {
			log.Warningf("network: took %d attempts to find a suitable unused port for pre-auth", i+1)
		}

		// The checks have passed. We have found a good unused port.
		return port, true
	}

	return 0, false
}

// portIsBindable reports whether the given local port can be bound, as the OS
// may have reserved it. Only UDP is checked, since checking TCP would mean
// listening on the port.
func portIsBindable(protocol uint8, port uint16) bool {
	if protocol != uint8(packet.UDP) {
		return true
	}

	conn, err := net.ListenPacket("udp", fmt.Sprintf(":%d", port))
	if err != nil {
		return false
	}
	_ = conn.Close()

	return true
}

// markPortUnusable remembers a port that could not be bound.
func markPortUnusable(protocol uint8, port uint16) {
	unusablePortsLock.Lock()
	defer unusablePortsLock.Unlock()

	unusablePorts[unusablePortKey(protocol, port)] = time.Now().Add(unusablePortTTL)
}

// portIsKnownUnusable reports whether the given port failed to bind recently.
func portIsKnownUnusable(protocol uint8, port uint16) bool {
	unusablePortsLock.Lock()
	defer unusablePortsLock.Unlock()

	key := unusablePortKey(protocol, port)
	expires, known := unusablePorts[key]
	if !known {
		return false
	}
	if time.Now().After(expires) {
		delete(unusablePorts, key)
		return false
	}

	return true
}

func unusablePortKey(protocol uint8, port uint16) uint32 {
	return uint32(protocol)<<16 | uint32(port)
}
