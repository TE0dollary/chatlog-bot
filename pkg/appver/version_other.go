//go:build !darwin

package appver

import "errors"

func (i *Info) initialize() error {
	return errors.New("unsupported platform")
}
