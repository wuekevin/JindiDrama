package mock

import (
	"fmt"
	"slices"
	"testing"

	"jindi-drama/internal/provider"
)

func TestHomeFeedReturnsEveryNewDramaInCatalogOrder(t *testing.T) {
	var p Provider
	var expected []string
	for i := range 10 {
		id := fmt.Sprintf("test-%d", i)
		isNew := i != 3 && i != 8
		p.dramas = append(p.dramas, provider.Drama{ID: id, IsNewToday: isNew})
		if isNew {
			expected = append(expected, id)
		}
	}
	feed, err := p.HomeFeed(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(feed.TodayNew, expected) {
		t.Fatalf("new releases = %v, want %v", feed.TodayNew, expected)
	}
}
