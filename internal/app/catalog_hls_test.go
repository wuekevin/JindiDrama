package app

import (
	"io"
	"net/http"
	"strconv"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"jindi-drama/internal/provider"
)

func issueTestHLS(t *testing.T, relay *catalogMediaRelay, expiry time.Time) provider.PlaybackTicket {
	t.Helper()
	ticket, err := relay.issue(provider.PlaybackTicket{Kind: "authorized", EpisodeID: "qa", Sources: []provider.MediaSource{
		{Src: "https://media.example.test/catalog/main.m3u8?signature=main-only-in-go", Type: catalogHLSType},
	}}, expiry)
	if err != nil {
		t.Fatal(err)
	}
	return ticket
}

func hlsTestResponse(status int, mediaType, body string) *http.Response {
	return &http.Response{
		StatusCode: status, Header: http.Header{"Content-Type": {mediaType}},
		ContentLength: int64(len(body)), Body: io.NopCloser(strings.NewReader(body)),
	}
}

func hlsTestRequest(t *testing.T, method, address string, headers http.Header) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(method, address, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header = headers
	res, err := (&http.Client{Timeout: 5 * time.Second}).Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	body, err := io.ReadAll(res.Body)
	if err != nil {
		t.Fatal(err)
	}
	return res, string(body)
}

const hlsVODStart = "#EXTM3U\n#EXT-X-TARGETDURATION:2\n"
const hlsVODEnd = "#EXT-X-ENDLIST\n"

var hlsTestTS = strings.Repeat("\x47"+strings.Repeat("\x00", 187), 3)

const hlsTestMP4 = "\x00\x00\x00\x10stypmsdh\x00\x00\x00\x00"

func TestCatalogHLSRewritesVariantAudioMapAndSegments(t *testing.T) {
	const master = `#EXTM3U
#EXT-X-VERSION:7
#https://tracker.example.test/comment
#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="audio",NAME="普通话",DEFAULT=YES,AUTOSELECT=YES,URI="https://audio.example.test/track?signature=audio"
#EXT-X-STREAM-INF:BANDWIDTH=350000,CODECS="avc1.42E01F,mp4a.40.2",AUDIO="audio"

#https://tracker.example.test/variant-comment

../video/stream.m3u8?signature=variant
`
	const video = hlsVODStart + `#EXT-X-VERSION:7
#EXT-X-KEY:METHOD=NONE
#EXT-X-ENDLIST
#EXT-X-MAP:URI="video.mp4?signature=payload",BYTERANGE="4@0"
#EXT-X-BYTERANGE:3@4
#EXTINF:2,title https://tracker.example.test/metadata
video.mp4?signature=payload
#EXTINF:2,
#EXT-X-BYTERANGE:3
video.mp4?signature=payload
`
	const audio = hlsVODStart + "#EXTINF:2,\ntrack.ts?signature=segment\n" + hlsVODEnd
	var calls atomic.Int32
	relay := testCatalogRelay(t, func(req *http.Request) (*http.Response, error) {
		calls.Add(1)
		if req.Header.Get("Origin") != "" || req.Header.Get("Cookie") != "" || req.Header.Get("Referer") != "" {
			t.Errorf("unexpected upstream headers: %v", req.Header)
		}
		switch req.URL.String() {
		case "https://media.example.test/catalog/main.m3u8?signature=main-only-in-go":
			if req.Method != http.MethodGet || req.Header.Get("Range") != "" {
				t.Error("upstream manifest must be requested in full")
			}
			return hlsTestResponse(http.StatusOK, "application/x-mpegURL", master), nil
		case "https://media.example.test/video/stream.m3u8?signature=variant":
			return hlsTestResponse(http.StatusOK, catalogHLSType, video), nil
		case "https://audio.example.test/track?signature=audio":
			return hlsTestResponse(http.StatusOK, "audio/mpegurl", audio), nil
		case "https://audio.example.test/track.ts?signature=segment":
			return hlsTestResponse(http.StatusOK, "video/mp2t", hlsTestTS), nil
		case "https://media.example.test/video/video.mp4?signature=payload":
			if req.Header.Get("Range") != "bytes=7-9" {
				t.Errorf("upstream Range: %q", req.Header.Get("Range"))
			}
			res := hlsTestResponse(http.StatusPartialContent, "video/iso.segment", "789")
			res.Header.Set("Content-Range", "bytes 7-9/10")
			return res, nil
		default:
			t.Errorf("unexpected request %s", req.URL)
			return hlsTestResponse(http.StatusNotFound, "text/html", "hidden"), nil
		}
	})
	ticket := issueTestHLS(t, relay, time.Now().Add(time.Minute))
	root := ticket.Sources[0].Src
	local := catalogMediaPath + ticket.LeaseID + "/hls/"
	origin := "http://" + relay.host
	headers := http.Header{"Origin": {catalogHLSOrigin}, "Cookie": {"synthetic=private"}, "Range": {"bytes=0-1"}}
	res, body := hlsTestRequest(t, http.MethodGet, root, headers)
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != catalogHLSType ||
		res.Header.Get("Access-Control-Allow-Origin") != catalogHLSOrigin || strings.Contains(body, "https:") ||
		strings.Contains(body, "signature") || !strings.Contains(body, `URI="`+local+`0.m3u8"`) ||
		!strings.Contains(body, `AUDIO="audio"`+"\n"+local+"1.m3u8\n") {
		t.Fatalf("unexpected master response: %d %v %q", res.StatusCode, res.Header, body)
	}
	head, headBody := hlsTestRequest(t, http.MethodHead, root, nil)
	if head.StatusCode != http.StatusOK || head.ContentLength != int64(len(body)) || headBody != "" {
		t.Fatalf("inconsistent manifest HEAD: %d %d %q", head.StatusCode, head.ContentLength, headBody)
	}
	res, body = hlsTestRequest(t, http.MethodGet, origin+local+"1.m3u8", http.Header{"Origin": {"http://wails.localhost.example"}})
	if res.StatusCode != http.StatusOK || res.Header.Get("Access-Control-Allow-Origin") != "" ||
		strings.Contains(body, "https:") || strings.Contains(body, "signature") ||
		strings.Count(body, local+"2") != 3 || !strings.Contains(body, "#EXT-X-BYTERANGE:3@7\n") ||
		!strings.Contains(body, "#EXT-X-KEY:METHOD=NONE\n") || !strings.HasSuffix(body, hlsVODEnd) {
		t.Fatalf("unexpected video manifest: %d %q", res.StatusCode, body)
	}
	res, body = hlsTestRequest(t, http.MethodGet, origin+local+"0.m3u8", nil)
	if res.StatusCode != http.StatusOK || !strings.Contains(body, local+"3\n") || strings.Contains(body, "signature") {
		t.Fatalf("unexpected audio manifest: %d %q", res.StatusCode, body)
	}
	res, body = hlsTestRequest(t, http.MethodGet, origin+local+"3", nil)
	if res.StatusCode != http.StatusOK || res.Header.Get("Content-Type") != "video/mp2t" || body != hlsTestTS {
		t.Fatalf("unexpected TS: %d %q", res.StatusCode, body)
	}
	options := http.Header{"Origin": {catalogHLSOrigin}, "Access-Control-Request-Method": {http.MethodGet}, "Access-Control-Request-Headers": {"range"}}
	res, _ = hlsTestRequest(t, http.MethodOptions, origin+local+"2", options)
	if res.StatusCode != http.StatusNoContent || res.Header.Get("Access-Control-Allow-Headers") != "Range" || calls.Load() != 5 {
		t.Fatalf("unexpected preflight: %d %v, upstream calls %d", res.StatusCode, res.Header, calls.Load())
	}
	res, body = hlsTestRequest(t, http.MethodGet, origin+local+"2", http.Header{"Origin": {catalogHLSOrigin}, "Range": {"bytes=7-9"}})
	if res.StatusCode != http.StatusPartialContent || res.Header.Get("Content-Range") != "bytes 7-9/10" ||
		res.Header.Get("Content-Type") != "video/mp4" || body != "789" {
		t.Fatalf("unexpected payload: %d %v %q", res.StatusCode, res.Header, body)
	}
	for _, invalid := range []string{origin + local + "4", origin + local + "2?url=https://different.example.test", origin + local + "2.m3u8"} {
		if res, _ = hlsTestRequest(t, http.MethodGet, invalid, nil); res.StatusCode != http.StatusNotFound {
			t.Fatalf("unknown mapping accepted: %d", res.StatusCode)
		}
	}
	relay.release(ticket.LeaseID)
	for _, address := range []string{root, origin + local + "0.m3u8", origin + local + "2"} {
		if res, _ = hlsTestRequest(t, http.MethodGet, address, nil); res.StatusCode != http.StatusNotFound || calls.Load() != 6 {
			t.Fatalf("released lease: %d, upstream calls %d", res.StatusCode, calls.Load())
		}
	}
}

func TestCatalogHLSRejectsUnsafeManifestWithoutPublishingResources(t *testing.T) {
	validSegment := "#EXTINF:2,\nvalid.ts?hidden=1\n"
	for _, tc := range []struct{ name, body string }{
		{"live without end", hlsVODStart + validSegment},
		{"unsupported low latency", hlsVODStart + validSegment + "#EXT-X-PART:DURATION=0.5,URI=\"part.ts\"\n" + hlsVODEnd},
		{"clear with key URI", hlsVODStart + validSegment + "#EXT-X-KEY:METHOD=NONE,URI=\"https://secret.example.test/key\"\n" + hlsVODEnd},
		{"quoted clear method", hlsVODStart + validSegment + "#EXT-X-KEY:METHOD=\"NONE\"\n" + hlsVODEnd},
		{"clear in master", "#EXTM3U\n#EXT-X-KEY:METHOD=NONE\n#EXT-X-STREAM-INF:BANDWIDTH=2000\nvideo.m3u8\n"},
		{"encryption before clear", hlsVODStart + "#EXT-X-KEY:METHOD=SAMPLE-AES,URI=\"key\"\n" + validSegment +
			"#EXT-X-KEY:METHOD=NONE\n" + hlsVODEnd},
		{"encryption", hlsVODStart + validSegment + "#EXT-X-KEY:METHOD=AES-128,URI=\"https://secret.example.test/key\"\n" + hlsVODEnd},
		{"variables", hlsVODStart + validSegment + "#EXT-X-DEFINE:NAME=\"upstream\",VALUE=\"secret\"\n" + hlsVODEnd},
		{"variable reference", hlsVODStart + validSegment + "#EXTINF:1,\n{$directory}/bad.ts\n" + hlsVODEnd},
		{"URI attribute", "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=2000,URI=\"../../tracker\"\nvideo.m3u8\n"},
		{"URI attribute in map", hlsVODStart + validSegment + "#EXT-X-MAP:URI=\"init.mp4\",X-URI=\"tracker\"\n" + hlsVODEnd},
		{"duplicate URI", hlsVODStart + validSegment + "#EXT-X-MAP:URI=\"init.mp4\",URI=\"tracker\"\n" + hlsVODEnd},
		{"network metadata", hlsVODStart + validSegment + "#EXT-X-DATERANGE:ID=\"ad\",X-ASSET-URI=\"tracking.m3u8\"\n" + hlsVODEnd},
		{"credentials", hlsVODStart + validSegment + "#EXTINF:1,\nhttps://user:pass@media.example.test/bad.ts\n" + hlsVODEnd},
		{"HTTP", hlsVODStart + validSegment + "#EXTINF:1,\nhttp://media.example.test/bad.ts\n" + hlsVODEnd},
		{"private IP", hlsVODStart + validSegment + "#EXTINF:1,\n//127.0.0.1/bad.ts\n" + hlsVODEnd},
		{"private host", hlsVODStart + validSegment + "#EXTINF:1,\n//printer.local/bad.ts\n" + hlsVODEnd},
		{"fragment", hlsVODStart + validSegment + "#EXTINF:1,\nbad.ts#hidden\n" + hlsVODEnd},
		{"missing implicit range predecessor", hlsVODStart + validSegment + "#EXTINF:1,\n#EXT-X-BYTERANGE:3\nother.ts\n" + hlsVODEnd},
		{"mixed master and media", hlsVODStart + validSegment + "#EXT-X-STREAM-INF:BANDWIDTH=2000\nvideo.m3u8\n" + hlsVODEnd},
		{"too many references", hlsVODStart + strings.Repeat(validSegment, maxCatalogHLSReferences+1) + hlsVODEnd},
		{"too many bytes", hlsVODStart + strings.Repeat("# comment\n", maxCatalogHLSManifest/10+1) + validSegment + hlsVODEnd},
		{"HTML mislabeled as HLS", "<html>private diagnostic</html>"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			var calls atomic.Int32
			relay := testCatalogRelay(t, func(req *http.Request) (*http.Response, error) {
				calls.Add(1)
				res := hlsTestResponse(http.StatusOK, catalogHLSType, tc.body)
				res.ContentLength = -1 // Exercise the streaming bytes limit too.
				return res, nil
			})
			ticket := issueTestHLS(t, relay, time.Now().Add(time.Minute))
			res, body := hlsTestRequest(t, http.MethodGet, ticket.Sources[0].Src, nil)
			if res.StatusCode != http.StatusBadGateway || body != "" || res.Header.Get("Content-Type") == catalogHLSType {
				t.Fatalf("unsafe response accepted: %d %q", res.StatusCode, body)
			}
			local := "http://" + relay.host + catalogMediaPath + ticket.LeaseID + "/hls/"
			for _, child := range []string{"0", "0.m3u8"} {
				res, _ = hlsTestRequest(t, http.MethodGet, local+child, nil)
				if res.StatusCode != http.StatusNotFound || calls.Load() != 1 {
					t.Fatalf("rejected manifest published a resource: %d; upstream calls %d", res.StatusCode, calls.Load())
				}
			}
		})
	}
}

func TestCatalogHLSRejectsNestedMastersRedirectsAndNonMediaTypes(t *testing.T) {
	const master = "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=2000\n"
	relay := testCatalogRelay(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/catalog/main.m3u8":
			return hlsTestResponse(http.StatusOK, catalogHLSType, master+"nested.m3u8\n"+
				"#EXT-X-STREAM-INF:BANDWIDTH=3000\nredirect.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=4000\nvideo.m3u8\n"), nil
		case "/catalog/nested.m3u8":
			return hlsTestResponse(http.StatusOK, catalogHLSType, master+"secret.m3u8\n"), nil
		case "/catalog/redirect.m3u8":
			res := hlsTestResponse(http.StatusFound, catalogHLSType, "hidden")
			res.Header.Set("Location", "https://another.example.test/secret")
			return res, nil
		case "/catalog/video.m3u8":
			return hlsTestResponse(http.StatusOK, catalogHLSType, hlsVODStart+"#EXTINF:1,\ninvalid.html\n"+hlsVODEnd), nil
		case "/catalog/invalid.html":
			return hlsTestResponse(http.StatusOK, "text/html", "secret error page"), nil
		default:
			t.Errorf("unexpected request %s", req.URL)
			return hlsTestResponse(http.StatusNotFound, "text/html", "hidden"), nil
		}
	})
	ticket := issueTestHLS(t, relay, time.Now().Add(time.Minute))
	res, _ := hlsTestRequest(t, http.MethodGet, ticket.Sources[0].Src, nil)
	if res.StatusCode != http.StatusOK {
		t.Fatalf("unexpected master status: %d", res.StatusCode)
	}
	local := "http://" + relay.host + catalogMediaPath + ticket.LeaseID + "/hls/"
	for _, name := range []string{"0.m3u8", "1.m3u8", "2.m3u8", "3"} {
		res, body := hlsTestRequest(t, http.MethodGet, local+name, nil)
		status := http.StatusBadGateway
		if name == "2.m3u8" {
			status = http.StatusOK
		}
		if res.StatusCode != status || res.Header.Get("Location") != "" || strings.Contains(body, "secret") {
			t.Fatalf("unexpected response for %s: %d %v %q", name, res.StatusCode, res.Header, body)
		}
	}
}

func TestCatalogHLSPublishResourceLimitIsAtomic(t *testing.T) {
	resourceStore := catalogHLSResources{index: make(map[hlsResource]int)}
	resourceStore.items = make([]hlsResource, maxCatalogHLSResources-1)
	var manifest hlsManifest
	for i := 0; i < 2; i++ {
		manifest.reference("", "", hlsResource{url: "https://media.example.test/" + strconv.Itoa(i)})
	}
	if body, ok := resourceStore.publish(manifest, "/test/"); ok || body != "" || len(resourceStore.index) != 0 ||
		len(resourceStore.items) != maxCatalogHLSResources-1 {
		t.Fatal("resource limit failed to reject manifest atomically")
	}
}

func TestCatalogHLSSameLeaseFallbackCanReuseURLInDifferentRole(t *testing.T) {
	const shared = "https://media.example.test/catalog/shared"
	relay := testCatalogRelay(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/catalog/master.m3u8":
			return hlsTestResponse(http.StatusOK, catalogHLSType, "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=2000\n"+shared+"\n"), nil
		case "/catalog/fallback.m3u8":
			return hlsTestResponse(http.StatusOK, catalogHLSType, hlsVODStart+"#EXTINF:2,\n"+shared+"\n"+hlsVODEnd), nil
		case "/catalog/shared":
			return hlsTestResponse(http.StatusOK, "video/mp2t", hlsTestTS), nil
		default:
			t.Errorf("unexpected request %s", req.URL)
			return hlsTestResponse(http.StatusNotFound, "text/html", "hidden"), nil
		}
	})
	ticket, err := relay.issue(provider.PlaybackTicket{Kind: "authorized", EpisodeID: "qa", Sources: []provider.MediaSource{
		{Src: "https://media.example.test/catalog/master.m3u8", Type: catalogHLSType},
		{Src: "https://media.example.test/catalog/fallback.m3u8", Type: catalogHLSType},
	}}, time.Now().Add(time.Minute))
	if err != nil {
		t.Fatal(err)
	}
	prefix := catalogMediaPath + ticket.LeaseID + "/hls/"
	local := "http://" + relay.host + prefix
	res, body := hlsTestRequest(t, http.MethodGet, ticket.Sources[0].Src, nil)
	if res.StatusCode != http.StatusOK || !strings.Contains(body, prefix+"0.m3u8") {
		t.Fatalf("unexpected first root: %d %q", res.StatusCode, body)
	}
	res, body = hlsTestRequest(t, http.MethodGet, local+"0.m3u8", nil)
	if res.StatusCode != http.StatusBadGateway || body != "" {
		t.Fatalf("invalid variant response: %d %q", res.StatusCode, body)
	}
	res, body = hlsTestRequest(t, http.MethodGet, ticket.Sources[1].Src, nil)
	if res.StatusCode != http.StatusOK || !strings.Contains(body, "\n"+prefix+"1\n") {
		t.Fatalf("fallback could not publish a segment using the same URL: %d %q", res.StatusCode, body)
	}
	res, body = hlsTestRequest(t, http.MethodGet, local+"1", nil)
	if res.StatusCode != http.StatusOK || body != hlsTestTS {
		t.Fatalf("fallback segment was not served: %d %q", res.StatusCode, body)
	}
}

func TestCatalogHLSAudioGroupMembers(t *testing.T) {
	type audioTrack struct{ group, name, defaultValue, route string }
	cantoneseTS := hlsTestTS[:1] + "C" + hlsTestTS[2:]
	mandarin := audioTrack{"stereo", "普通话", "YES", "mandarin"}
	cantonese := audioTrack{"stereo", "粤语", "NO", "cantonese"}
	for _, test := range []struct {
		name       string
		tracks     []audioTrack
		wantStatus int
	}{
		{"distinct members", []audioTrack{mandarin, cantonese}, http.StatusOK},
		{"same name in different groups", []audioTrack{mandarin, cantonese,
			{"surround", "普通话", "YES", "surround-mandarin"},
			{"surround", "粤语", "NO", "surround-cantonese"}}, http.StatusOK},
		{"duplicate name", []audioTrack{mandarin, {"stereo", "普通话", "NO", "cantonese"}}, http.StatusBadGateway},
		{"duplicate default", []audioTrack{mandarin, {"stereo", "粤语", "YES", "cantonese"}}, http.StatusBadGateway},
	} {
		t.Run(test.name, func(t *testing.T) {
			master := "#EXTM3U\n"
			for _, track := range test.tracks {
				master += `#EXT-X-MEDIA:TYPE=AUDIO,GROUP-ID="` + track.group + `",NAME="` + track.name +
					`",DEFAULT=` + track.defaultValue + `,URI="` + track.route + ".m3u8?private=fixture\"\n"
			}
			master += "#EXT-X-STREAM-INF:BANDWIDTH=350000,AUDIO=\"stereo\"\nvideo.m3u8\n"
			requests := make(chan string, 8)
			relay := testCatalogRelay(t, func(req *http.Request) (*http.Response, error) {
				requests <- req.URL.Path
				switch req.URL.Path {
				case "/catalog/main.m3u8":
					return hlsTestResponse(http.StatusOK, catalogHLSType, master), nil
				case "/catalog/mandarin.m3u8", "/catalog/cantonese.m3u8":
					segment := strings.TrimSuffix(strings.TrimPrefix(req.URL.Path, "/catalog/"), ".m3u8") + ".ts"
					return hlsTestResponse(http.StatusOK, catalogHLSType, hlsVODStart+"#EXTINF:2,\n"+segment+"\n"+hlsVODEnd), nil
				case "/catalog/mandarin.ts":
					return hlsTestResponse(http.StatusOK, "video/mp2t", hlsTestTS), nil
				case "/catalog/cantonese.ts":
					return hlsTestResponse(http.StatusOK, "video/mp2t", cantoneseTS), nil
				default:
					t.Errorf("unexpected request %s", req.URL)
					return hlsTestResponse(http.StatusNotFound, "text/plain", ""), nil
				}
			})
			ticket := issueTestHLS(t, relay, time.Now().Add(time.Minute))
			prefix := catalogMediaPath + ticket.LeaseID + "/hls/"
			local := "http://" + relay.host + prefix
			response, body := hlsTestRequest(t, http.MethodGet, ticket.Sources[0].Src, nil)
			if response.StatusCode != test.wantStatus || len(requests) != 1 || <-requests != "/catalog/main.m3u8" {
				t.Fatalf("master response: %d, want %d", response.StatusCode, test.wantStatus)
			}
			if test.wantStatus != http.StatusOK {
				response, _ := hlsTestRequest(t, http.MethodGet, local+"0.m3u8", nil)
				if body != "" || response.StatusCode != http.StatusNotFound || len(requests) != 0 {
					t.Fatalf("rejected master exposed a child: %d %q, extra requests=%d", response.StatusCode, body, len(requests))
				}
				return
			}
			for index, track := range test.tracks {
				location := prefix + strconv.Itoa(index) + ".m3u8"
				if !strings.Contains(body, `NAME="`+track.name+`",DEFAULT=`+track.defaultValue+`,URI="`+location+`"`) ||
					strings.Contains(body, "private=fixture") {
					t.Fatalf("unexpected rewritten master: %q", body)
				}
			}
			for index, track := range test.tracks[:2] {
				response, child := hlsTestRequest(t, http.MethodGet, local+strconv.Itoa(index)+".m3u8", nil)
				if response.StatusCode != http.StatusOK || len(requests) != 1 || <-requests != "/catalog/"+track.route+".m3u8" {
					t.Fatalf("wrong child for %s: %d", track.name, response.StatusCode)
				}
				mediaPath := prefix + strconv.Itoa(len(test.tracks)+1+index)
				if !strings.Contains(child, mediaPath) {
					t.Fatalf("missing relayed segment: %q", child)
				}
				payload, segment := hlsTestRequest(t, http.MethodGet, "http://"+relay.host+mediaPath, nil)
				expectedSegment := hlsTestTS
				if index == 1 {
					expectedSegment = cantoneseTS
				}
				if payload.StatusCode != http.StatusOK || segment != expectedSegment ||
					len(requests) != 1 || <-requests != "/catalog/"+track.route+".ts" {
					t.Fatalf("incomplete TS segment for %s: %d", track.name, payload.StatusCode)
				}
			}
		})
	}
}

func TestCatalogHLSGenericManifestMIMERequiresM3U8AndValidBody(t *testing.T) {
	const master = "#EXTM3U\n#EXT-X-STREAM-INF:BANDWIDTH=2000\nvalid.M3U8\n" +
		"#EXT-X-STREAM-INF:BANDWIDTH=2000\nhtml.m3u8\n#EXT-X-STREAM-INF:BANDWIDTH=2000\nextensionless\n"
	const media = hlsVODStart + "#EXTINF:2,\nsegment.ts\n" + hlsVODEnd
	relay := testCatalogRelay(t, func(req *http.Request) (*http.Response, error) {
		switch req.URL.Path {
		case "/catalog/main.m3u8":
			return hlsTestResponse(http.StatusOK, "application/octet-stream", master), nil
		case "/catalog/valid.M3U8", "/catalog/extensionless":
			return hlsTestResponse(http.StatusOK, "text/plain; charset=utf-8", media), nil
		case "/catalog/html.m3u8":
			return hlsTestResponse(http.StatusOK, "text/plain", "<html>private diagnostic</html>"), nil
		case "/catalog/segment.ts":
			return hlsTestResponse(http.StatusOK, "application/octet-stream", hlsTestTS), nil
		default:
			t.Errorf("unexpected request %s", req.URL)
			return hlsTestResponse(http.StatusNotFound, "text/html", "hidden"), nil
		}
	})
	ticket := issueTestHLS(t, relay, time.Now().Add(time.Minute))
	local := "http://" + relay.host + catalogMediaPath + ticket.LeaseID + "/hls/"
	for _, tc := range []struct {
		address string
		status  int
	}{
		{ticket.Sources[0].Src, http.StatusOK},
		{local + "0.m3u8", http.StatusOK},
		{local + "1.m3u8", http.StatusBadGateway},
		{local + "2.m3u8", http.StatusBadGateway},
		{local + "3", http.StatusBadGateway},
	} {
		res, body := hlsTestRequest(t, http.MethodGet, tc.address, nil)
		if res.StatusCode != tc.status || (tc.status == http.StatusOK && res.Header.Get("Content-Type") != catalogHLSType) ||
			(tc.status != http.StatusOK && body != "") {
			t.Fatalf("unexpected response for %s: %d %v %q", tc.address, res.StatusCode, res.Header, body)
		}
	}
}

func TestCatalogHLSChecksPayloadPrefixBeforeWritingResponse(t *testing.T) {
	const secret = "https://media.example.test/segment?signature=private-upstream-diagnostic"
	const html = "\xef\xbb\xbf<html><body>" + secret + "</body></html>"
	for _, tc := range []struct {
		name, mediaType, method, byteRange, contentRange, body string
		upstream, expected                                     int
	}{
		{"full HTML as TS", "video/mp2t", http.MethodGet, "", "", html, http.StatusOK, http.StatusBadGateway},
		{"full text as MP4", "video/mp4", http.MethodGet, "", "", secret, http.StatusOK, http.StatusBadGateway},
		{"nonzero HTML", "video/mp4", http.MethodGet, "bytes=5-", "bytes 5-" + strconv.Itoa(4+len(html)) + "/" + strconv.Itoa(5+len(html)), html, http.StatusPartialContent, http.StatusBadGateway},
		{"zero range text as MP4", "video/mp4", http.MethodGet, "bytes=0-", "bytes 0-" + strconv.Itoa(len(secret)-1) + "/" + strconv.Itoa(len(secret)), secret, http.StatusPartialContent, http.StatusBadGateway},
		{"full fMP4", "video/iso.segment", http.MethodGet, "", "", hlsTestMP4, http.StatusOK, http.StatusOK},
		{"short MP4 probe", "video/mp4", http.MethodGet, "bytes=0-1", "bytes 0-1/16", hlsTestMP4[:2], http.StatusPartialContent, http.StatusPartialContent},
		{"short TS probe", "video/mp2t", http.MethodGet, "bytes=0-1", "bytes 0-1/564", hlsTestTS[:2], http.StatusPartialContent, http.StatusPartialContent},
		{"HEAD media metadata", "video/mp4", http.MethodHead, "", "", hlsTestMP4, http.StatusOK, http.StatusOK},
	} {
		t.Run(tc.name, func(t *testing.T) {
			relay := testCatalogRelay(t, func(req *http.Request) (*http.Response, error) {
				if req.URL.Path == "/catalog/main.m3u8" {
					return hlsTestResponse(http.StatusOK, catalogHLSType, hlsVODStart+"#EXTINF:2,\nsegment\n"+hlsVODEnd), nil
				}
				if req.URL.Path != "/catalog/segment" || req.Method != tc.method || req.Header.Get("Range") != tc.byteRange {
					t.Errorf("unexpected upstream payload request: %s %s %q", req.Method, req.URL.Path, req.Header.Get("Range"))
				}
				res := hlsTestResponse(tc.upstream, tc.mediaType, tc.body)
				if tc.contentRange != "" {
					res.Header.Set("Content-Range", tc.contentRange)
				}
				return res, nil
			})
			ticket := issueTestHLS(t, relay, time.Now().Add(time.Minute))
			if res, _ := hlsTestRequest(t, http.MethodGet, ticket.Sources[0].Src, nil); res.StatusCode != http.StatusOK {
				t.Fatalf("unexpected manifest status: %d", res.StatusCode)
			}
			local := "http://" + relay.host + catalogMediaPath + ticket.LeaseID + "/hls/0"
			headers := make(http.Header)
			if tc.byteRange != "" {
				headers.Set("Range", tc.byteRange)
			}
			res, body := hlsTestRequest(t, tc.method, local, headers)
			if res.StatusCode != tc.expected {
				t.Fatalf("unexpected payload response: %d %v %q", res.StatusCode, res.Header, body)
			}
			if tc.expected == http.StatusBadGateway {
				if body != "" || res.Header.Get("Content-Type") != "" || res.Header.Get("Content-Range") != "" {
					t.Fatalf("bad payload leaked: %v %q", res.Header, body)
				}
			} else if tc.method == http.MethodHead {
				if body != "" || res.ContentLength != int64(len(tc.body)) {
					t.Fatalf("HEAD response was changed: %v %q", res.Header, body)
				}
			} else if body != tc.body || res.Header.Get("Content-Range") != tc.contentRange {
				t.Fatalf("valid payload or range was changed: %v %q", res.Header, body)
			}
		})
	}
}
