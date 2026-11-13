//go:build !windows

package main

import "errors"

func seal(data, entropy []byte) ([]byte, error)   { return nil, errors.New("Windows DPAPI required") }
func unseal(data, entropy []byte) ([]byte, error) { return nil, errors.New("Windows DPAPI required") }
