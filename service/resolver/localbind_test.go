package resolver

import (
	"net"
	"os"
	"syscall"
	"testing"
)

const (
	testOpDial = "dial"
	testNetUDP = "udp"
	testOpRead = "read"
)

func TestIsLocalBindError(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name string
		err  error
		want bool
	}{
		{
			name: "nil",
			err:  nil,
			want: false,
		},
		{
			name: "bind denied",
			err: &net.OpError{
				Op:  testOpDial,
				Net: testNetUDP,
				Err: os.NewSyscallError("bind", syscall.EACCES),
			},
			want: true,
		},
		{
			name: "bind in use",
			err: &net.OpError{
				Op:  testOpDial,
				Net: testNetUDP,
				Err: os.NewSyscallError("bind", syscall.EADDRINUSE),
			},
			want: true,
		},
		{
			name: "connect refused",
			err: &net.OpError{
				Op:  testOpDial,
				Net: testNetUDP,
				Err: os.NewSyscallError("connect", syscall.ECONNREFUSED),
			},
			want: false,
		},
		{
			name: "timeout",
			err: &net.OpError{
				Op:  testOpRead,
				Net: testNetUDP,
				Err: os.ErrDeadlineExceeded,
			},
			want: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			if got := isLocalBindError(tc.err); got != tc.want {
				t.Errorf("isLocalBindError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
