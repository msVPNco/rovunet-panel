//go:build windows

package main

import (
	"errors"
	"golang.org/x/sys/windows"
	"runtime"
	"unsafe"
)

func protect(data, entropy []byte, encrypt bool) ([]byte, error) {
	if len(data) == 0 || len(entropy) == 0 {
		return nil, errors.New("empty DPAPI input")
	}
	in := windows.DataBlob{Size: uint32(len(data)), Data: &data[0]}
	context := windows.DataBlob{Size: uint32(len(entropy)), Data: &entropy[0]}
	var out windows.DataBlob
	var err error
	if encrypt {
		err = windows.CryptProtectData(&in, nil, &context, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	} else {
		err = windows.CryptUnprotectData(&in, nil, &context, 0, nil, windows.CRYPTPROTECT_UI_FORBIDDEN, &out)
	}
	runtime.KeepAlive(data)
	runtime.KeepAlive(entropy)
	if err != nil {
		return nil, err
	}
	defer windows.LocalFree(windows.Handle(uintptr(unsafe.Pointer(out.Data))))
	plain := unsafe.Slice(out.Data, int(out.Size))
	defer clear(plain)
	return append([]byte(nil), plain...), nil
}

func seal(data, entropy []byte) ([]byte, error)   { return protect(data, entropy, true) }
func unseal(data, entropy []byte) ([]byte, error) { return protect(data, entropy, false) }
