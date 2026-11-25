package main

import (
	"slices"
	"testing"
)

func TestShareLinksQueueUntilDesktopAndFrontendAreReady(t *testing.T) {
	links := newShareLinks()
	var got []string
	links.openArgs([]string{"jindi://drama/d1"})
	links.open("jindi://watch/d2/4")
	links.open("jindi://drama/d999")
	links.open("jindi://watch/d2/9999")
	links.frontendReady()
	links.start(func(route string) { got = append(got, route) })
	links.frontendReady()
	links.openArgs([]string{"jindi://drama/d3"})
	for _, invalid := range []string{
		"https://example.com/drama/d1", "jindi://settings/d1", "jindi://drama/local-test",
		"jindi://drama/d1?arg=1", "jindi://watch/d1/0", "jindi://watch/d1/01",
		"jindi://watch/d1/2#fragment", "jindi://watch/d1/2\n", "jindi://watch/d1/%32",
	} {
		links.open(invalid)
	}
	links.openArgs([]string{"jindi://drama/d4", "unexpected"})
	if want := []string{"/watch/d2/4", "/drama/d3"}; !slices.Equal(got, want) {
		t.Fatalf("delivered: %v, want %v", got, want)
	}
}

func TestPlainWindowsRelaunchRevealsWithoutReplacingPendingShareLink(t *testing.T) {
	links := newShareLinks()
	var got []string
	links.revealWindow()
	links.open("jindi://drama/d1")
	links.revealWindow()
	links.start(func(route string) { got = append(got, route) })
	if len(got) != 0 {
		t.Fatal("window action delivered before frontend readiness")
	}
	links.frontendReady()
	links.revealWindow()
	if want := []string{"/drama/d1", ""}; !slices.Equal(got, want) {
		t.Fatalf("delivered: %v, want %v", got, want)
	}
}
