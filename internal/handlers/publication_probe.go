package handlers

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

var probePublished = func(host, token string) error {
	client := http.Client{Timeout: 4 * time.Second, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	return probeAliases(&client, "https://"+host, token)
}

func probeAliases(client *http.Client, base, token string) error {
	for _, suffix := range aliasSuffixes {
		expected, e := os.ReadFile(filepath.Join(publicDir(), token+suffix))
		if e != nil {
			return e
		}
		resp, e := client.Get(base + "/sub/" + token + suffix)
		if e != nil {
			return errors.New("HTTPS subscription verification failed")
		}
		b, e := io.ReadAll(io.LimitReader(resp.Body, int64(len(expected))+1))
		resp.Body.Close()
		if e != nil || resp.StatusCode != 200 || !bytes.Equal(b, expected) || resp.Header.Get("Cache-Control") != "no-store" {
			return errors.New("served subscription snapshot disagrees")
		}
	}
	return nil
}
