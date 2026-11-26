package reference_test

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"jindi-drama/internal/app"
	"jindi-drama/internal/provider"
	"jindi-drama/internal/provider/reference"
)

func TestDesktopCatalogThroughReferenceHTTPContracts(t *testing.T) {
	for _, name := range []string{reference.OpenPlayer, reference.DramaServer} {
		t.Run(name, func(t *testing.T) {
			const clip = `{"series_id":"12345678901234567890","series_title":"样片","total":4,"intro":"详情","episodes":[{"vid":"9002","vid_index":2},{"vid":"9001","vid_index":1}]}`
			const results = `{"success":true,"results":[{"series_id":"12345678901234567890","series_title":"样片","episode_count":4,"description":"搜索简介"}]}`
			token, query := "", "keyword"
			if name == reference.DramaServer {
				token, query = "fixture-token-of-at-least-32-characters", "q"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				auth := ""
				if token != "" {
					auth = "Bearer " + token
				}
				if r.Method != http.MethodGet || r.Header.Get("Authorization") != auth {
					t.Errorf("unexpected method or authorization")
					http.Error(w, "unauthorized", http.StatusUnauthorized)
					return
				}
				if name == reference.OpenPlayer {
					if r.URL.Path == "/" {
						http.SetCookie(w, &http.Cookie{Name: "player_session", Value: "test-session", Path: "/", HttpOnly: true})
						fmt.Fprint(w, "Open Player")
						return
					}
					session, err := r.Cookie("player_session")
					if err != nil || session.Value != "test-session" {
						http.Error(w, "session required", http.StatusUnauthorized)
						return
					}
				}
				response := results
				if name == reference.DramaServer {
					response = `{"success":true,"results":[{"series_id":"12345678901234567890","series_title":"样片"}]}`
				}
				switch r.URL.Path {
				case "/api/catalog":
					if name != reference.OpenPlayer {
						t.Errorf("Docker service has no catalog endpoint")
					}
					fmt.Fprint(w, response)
				case "/api/search":
					if r.URL.Query().Get(query) != "样片 & 夜" || r.URL.Query().Get("page") != "1" {
						t.Errorf("incorrect search parameters: %s", r.URL.RawQuery)
					}
					fmt.Fprint(w, response)
				case "/api/series/12345678901234567890":
					if name == reference.DramaServer {
						fmt.Fprint(w, `{"success":true,"series":{"series_id":"12345678901234567890","series_title":"样片","total":2,"episodes":[{"vid":"9001","vid_index":1},{"vid":"9002","vid_index":2}]}}`)
					} else {
						fmt.Fprint(w, clip)
					}
				default:
					t.Errorf("unexpected route: %s", r.URL.Path)
					http.NotFound(w, r)
				}
			}))
			defer server.Close()
			p, err := reference.New(name, server.URL, token)
			if err != nil {
				t.Fatal(err)
			}
			a := app.New()
			app.ConfigureCatalogProvider(a, p)
			if a.ProviderName() != name || a.Capabilities()["playback"] != provider.Unsupported {
				t.Fatal("incorrect active provider or playback capability")
			}
			home, err := a.Home()
			wantHome := 0
			if name == reference.OpenPlayer {
				wantHome = 1
			}
			if err != nil || len(home) != wantHome {
				t.Fatalf("home: %v %v", home, err)
			}
			found, err := a.CatalogSearch(" 样片 & 夜 ")
			if err != nil || len(found) != 1 || (name == reference.OpenPlayer && found[0].Summary != "搜索简介") {
				t.Fatalf("search: %#v %v", found, err)
			}
			detail, err := a.CatalogDrama(found[0].ID)
			if err != nil || detail.UpdatedTo != 2 || (name == reference.OpenPlayer && (detail.Summary != "详情" || detail.EpisodeCount != 4)) || (name == reference.DramaServer && detail.EpisodeCount != 2) {
				t.Fatalf("detail: %#v %v", detail, err)
			}
			episodes, err := a.CatalogEpisodes(detail.ID)
			if err != nil || len(episodes) != 2 || episodes[0].ID != "9001" || !episodes[0].Available || !episodes[1].Available {
				t.Fatalf("episodes: %#v %v", episodes, err)
			}
			if _, err := a.CatalogPlayback(detail.ID, 1); err == nil {
				t.Fatal("unexpected playback capability")
			}
		})
	}
}

func TestReferenceRejectsRedirectAndInvalidConfiguration(t *testing.T) {
	requests := 0
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		fmt.Fprint(w, `{"success":true,"results":[]}`)
	}))
	defer sink.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, sink.URL, http.StatusTemporaryRedirect)
	}))
	defer server.Close()
	p, err := reference.New(reference.DramaServer, server.URL, "fixture-token-of-at-least-32-characters")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = p.Search(context.Background(), "test"); err == nil || requests != 0 {
		t.Fatalf("redirect propagated: %v %d", err, requests)
	}
	for _, address := range []string{"http://example.com", "ftp://127.0.0.1", server.URL + "?secret=x", server.URL + "#x", strings.Replace(server.URL, "://", "://user@", 1)} {
		if _, err := reference.New(reference.DramaServer, address, "fixture-token-of-at-least-32-characters"); err == nil {
			t.Errorf("accepted malformed URL: %s", address)
		}
	}
	if _, err := reference.New(reference.DramaServer, server.URL, ""); err == nil {
		t.Fatal("accepted missing bearer token")
	}
}

func TestReferenceRejectsUnboundedEpisodeCountsAndSearchWords(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/api/series/123":
			fmt.Fprint(w, `{"series_id":"123","series_title":"样片","total":1073741824,"episodes":[]}`)
		case "/api/series/456":
			fmt.Fprint(w, `{"series_id":"456","series_title":"样片","episodes":[{"vid":"100","vid_index":1073741824}]}`)
		default:
			t.Errorf("unexpected route: %s", r.URL.Path)
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	p, err := reference.New(reference.OpenPlayer, server.URL, "")
	if err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"123", "456"} {
		if _, err := p.Drama(context.Background(), id); err == nil {
			t.Errorf("accepted unbounded episode count for %s", id)
		}
	}
	if _, err := p.Search(context.Background(), strings.Repeat("🧪", 41)); err == nil {
		t.Fatal("accepted an Open Player keyword longer than 80 UTF-16 units")
	}
}
