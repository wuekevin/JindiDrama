//go:build !windows

package main

import "errors"

func runNative([]string) error {
	return errors.New("native window inspection requires a Windows desktop")
}
