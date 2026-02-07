//go:build linux
// +build linux

package fileinfo

import (
	"errors"
	"net"
	"testing"
)

func TestIsBenignNetworkCloseError(t *testing.T) {
	if !isBenignNetworkCloseError(net.ErrClosed) {
		t.Fatalf("net.ErrClosed should be treated as benign")
	}
	if !isBenignNetworkCloseError(errors.New("use of closed network connection")) {
		t.Fatalf("closed network connection text should be treated as benign")
	}
	if isBenignNetworkCloseError(errors.New("permission denied")) {
		t.Fatalf("non-close errors should not be treated as benign")
	}
}
