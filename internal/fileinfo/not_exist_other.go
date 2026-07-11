//go:build !linux

package fileinfo

func isProviderNotExist(error) bool {
	return false
}
