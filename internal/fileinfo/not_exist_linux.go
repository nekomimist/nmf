//go:build linux

package fileinfo

import (
	"errors"

	"github.com/hirochachacha/go-smb2"
)

const (
	smbStatusNoSuchFile        = 0xC000000F
	smbStatusObjectNameMissing = 0xC0000034
	smbStatusObjectPathMissing = 0xC000003A
)

func isProviderNotExist(err error) bool {
	var responseErr *smb2.ResponseError
	if !errors.As(err, &responseErr) {
		return false
	}
	switch responseErr.Code {
	case smbStatusNoSuchFile, smbStatusObjectNameMissing, smbStatusObjectPathMissing:
		return true
	default:
		return false
	}
}
