package handlers

import (
	"fmt"
	"strings"
	"testing"
)

func TestPagedTunnelsShowsAllThirteen(t *testing.T) {
	var all []tunnelInfo
	for i := 0; i < 13; i++ {
		all = append(all, tunnelInfo{tag: fmt.Sprintf("tunnel-%d", i)})
	}
	first, firstPage, pages := pagedTunnels(all, 0, 10)
	if len(first) != 10 || firstPage != 0 || pages != 2 {
		t.Fatalf("first page: count=%d page=%d pages=%d", len(first), firstPage, pages)
	}
	last, lastPage, _ := pagedTunnels(all, 1, 10)
	if len(last) != 3 || lastPage != 1 || last[2].tag != "tunnel-12" {
		t.Fatalf("last page: %+v (%d)", last, lastPage)
	}
	over, page, _ := pagedTunnels(all, 99, 10)
	if page != 1 || len(over) != 3 {
		t.Fatal("out-of-range page was not clamped")
	}
	none, _, pages := pagedTunnels(nil, 0, 10)
	if len(none) != 0 || pages != 0 {
		t.Fatal("empty tunnel list must not invent pages")
	}
}

func TestParseInterfaceTrafficIncludesTwoUplinksButNotTun(t *testing.T) {
	line := func(name string, rx, tx uint64) string {
		return fmt.Sprintf(" %s: %d 0 0 0 0 0 0 0 %d 0 0 0 0 0 0 0\n", name, rx, tx)
	}
	sample := "Inter-| Receive | Transmit\n face | bytes ...\n" +
		line("lo", 999, 999) + line("eth0", 100, 200) + line("eth1", 300, 400) +
		line("tun0", 2000, 2000) + line("veth777", 4000, 4000)
	rx, tx := parseInterfaceTraffic(strings.NewReader(sample))
	if rx != 400 || tx != 600 {
		t.Fatalf("uplink RX/TX %d/%d; want 400/600", rx, tx)
	}
}
