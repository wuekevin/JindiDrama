package app

import (
	"errors"
	"math"
	"net/url"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"
)

const (
	catalogHLSType          = "application/vnd.apple.mpegurl"
	maxCatalogHLSManifest   = 512 << 10
	maxCatalogHLSURL        = 4096
	maxCatalogHLSReferences = 2048
	maxCatalogHLSResources  = 4096
)

var errCatalogHLSManifest = errors.New("无效的授权播放清单")

type hlsResource struct {
	url      string
	playlist bool
}

type hlsLine struct {
	prefix, suffix string
	resource       hlsResource
}

type hlsManifest struct {
	lines      []hlsLine
	references int
}

func (m *hlsManifest) literal(line string) {
	m.lines = append(m.lines, hlsLine{prefix: line})
}

func (m *hlsManifest) reference(prefix, suffix string, resource hlsResource) {
	m.lines = append(m.lines, hlsLine{prefix: prefix, suffix: suffix, resource: resource})
	m.references++
}

func resolveHLSResource(base *url.URL, raw string, playlist bool) (hlsResource, bool) {
	if raw == "" || len(raw) > maxCatalogHLSURL || strings.ContainsAny(raw, "\\#") ||
		strings.Contains(raw, "{$") || strings.IndexFunc(raw, unicode.IsSpace) >= 0 {
		return hlsResource{}, false
	}
	ref, err := url.Parse(raw)
	if err != nil || (ref.Scheme != "" && (ref.Scheme != "https" || ref.Host == "")) {
		return hlsResource{}, false
	}
	address := base.ResolveReference(ref).String()
	_, ok := catalogHTTPSURL(address)
	return hlsResource{url: address, playlist: playlist}, ok && len(address) <= maxCatalogHLSURL
}

type hlsRange struct {
	length, offset int64
	explicit       bool
}

func parseHLSRange(value string) (hlsRange, bool) {
	size, start, explicit := strings.Cut(value, "@")
	length, sizeOK := parseCatalogOffset(size)
	offset := int64(0)
	if explicit {
		var ok bool
		offset, ok = parseCatalogOffset(start)
		if !ok {
			return hlsRange{}, false
		}
	}
	return hlsRange{length: length, offset: offset, explicit: explicit}, sizeOK && length > 0 && offset <= math.MaxInt64-length
}

type hlsManifestState struct {
	manifest hlsManifest
	base     *url.URL
	// A child playlist always has the media role. Only a ticket root can be a master.
	allowMaster       bool
	master, media     bool
	pendingVariant    bool
	pendingSegment    bool
	ended             bool
	discontinuitySeen bool
	variants          int
	segments          int
	version           int64
	minVersion        int64
	targetDuration    float64
	maxDuration       float64
	pendingRange      hlsRange
	previousRangeEnd  int64
	previousURL       string
	singletons        map[string]bool
	audioGroups       map[string]bool
	referencedAudio   map[string]bool
}

func parseCatalogHLSManifest(body []byte, base *url.URL, allowMaster bool) (hlsManifest, error) {
	if len(body) > maxCatalogHLSManifest || !utf8.Valid(body) {
		return hlsManifest{}, errCatalogHLSManifest
	}
	content := strings.ReplaceAll(string(body), "\r\n", "\n")
	if strings.IndexFunc(content, func(r rune) bool { return r != '\n' && unicode.IsControl(r) }) >= 0 {
		return hlsManifest{}, errCatalogHLSManifest
	}
	header, rest, _ := strings.Cut(content, "\n")
	if header != "#EXTM3U" {
		return hlsManifest{}, errCatalogHLSManifest
	}
	s := hlsManifestState{
		base: base, allowMaster: allowMaster, version: 1, singletons: make(map[string]bool),
		audioGroups: make(map[string]bool), referencedAudio: make(map[string]bool),
	}
	s.manifest.literal(header)
	for line := range strings.SplitSeq(rest, "\n") {
		if len(line) > 2*maxCatalogHLSURL || !s.accept(line) || s.manifest.references > maxCatalogHLSReferences {
			return hlsManifest{}, errCatalogHLSManifest
		}
	}
	if s.pendingVariant || s.pendingSegment || s.pendingRange.length != 0 || s.version < s.minVersion {
		return hlsManifest{}, errCatalogHLSManifest
	}
	if s.master {
		if s.variants == 0 {
			return hlsManifest{}, errCatalogHLSManifest
		}
		for group := range s.referencedAudio {
			if !s.audioGroups[group] {
				return hlsManifest{}, errCatalogHLSManifest
			}
		}
		return s.manifest, nil
	}
	if !s.ended || s.segments == 0 || s.targetDuration <= 0 || math.Floor(s.maxDuration+0.5) > s.targetDuration {
		return hlsManifest{}, errCatalogHLSManifest
	}
	s.manifest.literal("#EXT-X-ENDLIST")
	return s.manifest, nil
}

func (s *hlsManifestState) accept(line string) bool {
	if line == "" || (strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "#EXT")) {
		return true // Discard comments and blank lines, including metadata with URLs.
	}
	if s.pendingVariant {
		resource, ok := resolveHLSResource(s.base, line, true)
		if !ok {
			return false
		}
		s.manifest.reference("", "", resource)
		s.pendingVariant = false
		s.variants++
		return true
	}
	if !strings.HasPrefix(line, "#") {
		return s.acceptSegment(line)
	}

	tag, value, hasValue := strings.Cut(line, ":")
	switch tag {
	case "#EXT-X-STREAM-INF", "#EXT-X-MEDIA":
		if !s.allowMaster || s.media || !hasValue || !s.acceptMasterAttributes(tag, value) {
			return false
		}
		s.master = true
		return true
	case "#EXTINF", "#EXT-X-BYTERANGE", "#EXT-X-MAP", "#EXT-X-DISCONTINUITY",
		"#EXT-X-TARGETDURATION", "#EXT-X-MEDIA-SEQUENCE", "#EXT-X-DISCONTINUITY-SEQUENCE",
		"#EXT-X-PLAYLIST-TYPE", "#EXT-X-ENDLIST":
		if s.master || !s.acceptMediaTag(tag, value, hasValue) {
			return false
		}
		s.media = true
		return true
	case "#EXT-X-VERSION":
		version, ok := parseCatalogOffset(value)
		if !hasValue || !ok || version < 1 || version > 7 {
			return false
		}
		s.version = version
	case "#EXT-X-INDEPENDENT-SEGMENTS":
		if hasValue {
			return false
		}
	case "#EXT-X-START":
		if !hasValue || !validHLSStart(value) {
			return false
		}
	default:
		return false
	}
	return s.unique(tag, line)
}

func (s *hlsManifestState) unique(tag, line string) bool {
	if s.singletons[tag] {
		return false
	}
	s.singletons[tag] = true
	s.manifest.literal(line)
	return true
}

func (s *hlsManifestState) acceptSegment(raw string) bool {
	if !s.pendingSegment {
		return false
	}
	resource, ok := resolveHLSResource(s.base, raw, false)
	if !ok {
		return false
	}
	byteRange := s.pendingRange
	s.pendingRange = hlsRange{}
	if byteRange.length == 0 {
		s.previousRangeEnd = 0
	} else {
		if !byteRange.explicit {
			if s.previousURL != resource.url || s.previousRangeEnd == 0 || s.previousRangeEnd > math.MaxInt64-byteRange.length {
				return false
			}
			byteRange.offset = s.previousRangeEnd
		}
		s.manifest.literal("#EXT-X-BYTERANGE:" + strconv.FormatInt(byteRange.length, 10) + "@" + strconv.FormatInt(byteRange.offset, 10))
		s.previousRangeEnd = byteRange.offset + byteRange.length
	}
	s.previousURL = resource.url
	s.manifest.reference("", "", resource)
	s.pendingSegment = false
	s.segments++
	return true
}

func (s *hlsManifestState) acceptMediaTag(tag, value string, hasValue bool) bool {
	switch tag {
	case "#EXTINF":
		durationText, _, comma := strings.Cut(value, ",")
		duration, ok := parseHLSDecimal(durationText, false)
		if !hasValue || !comma || !ok || s.pendingSegment {
			return false
		}
		s.maxDuration = max(s.maxDuration, duration)
		if strings.Contains(durationText, ".") {
			s.minVersion = max(s.minVersion, 3)
		}
		s.pendingSegment = true
		s.manifest.literal(tag + ":" + durationText + ",")
		return true
	case "#EXT-X-BYTERANGE":
		byteRange, ok := parseHLSRange(value)
		if !hasValue || !ok || s.pendingRange.length != 0 {
			return false
		}
		s.minVersion = max(s.minVersion, 4)
		s.pendingRange = byteRange
		return true
	case "#EXT-X-MAP":
		s.minVersion = max(s.minVersion, 6)
		return hasValue && s.acceptMap(value)
	case "#EXT-X-DISCONTINUITY":
		if hasValue {
			return false
		}
		s.discontinuitySeen = true
		s.manifest.literal(tag)
		return true
	case "#EXT-X-ENDLIST":
		if hasValue || s.ended {
			return false
		}
		s.ended = true
		return true
	case "#EXT-X-TARGETDURATION":
		number, ok := parseCatalogOffset(value)
		if !hasValue || !ok {
			return false
		}
		s.targetDuration = float64(number)
	case "#EXT-X-MEDIA-SEQUENCE", "#EXT-X-DISCONTINUITY-SEQUENCE":
		if _, ok := parseCatalogOffset(value); !hasValue || !ok || s.segments != 0 ||
			(tag == "#EXT-X-DISCONTINUITY-SEQUENCE" && s.discontinuitySeen) {
			return false
		}
	case "#EXT-X-PLAYLIST-TYPE":
		if !hasValue || (value != "VOD" && value != "EVENT") {
			return false
		}
	}
	return s.unique(tag, tag+":"+value)
}

func parseHLSDecimal(value string, signed bool) (float64, bool) {
	digits := value
	if signed {
		digits = strings.TrimPrefix(digits, "-")
	}
	whole, fraction, dot := strings.Cut(digits, ".")
	if _, ok := parseCatalogOffset(whole); !ok {
		return 0, false
	}
	if _, ok := parseCatalogOffset(fraction); dot && !ok {
		return 0, false
	}
	n, err := strconv.ParseFloat(value, 64)
	return n, err == nil
}

// Only the RFC 8216 attributes checked below can reach a client. Separate the
// quoted and unquoted grammars so an attribute cannot introduce another field.
type hlsAttribute struct {
	name, value string
	quoted      bool
}

func parseHLSAttributes(raw string) ([]hlsAttribute, bool) {
	var attributes []hlsAttribute
	seen := make(map[string]bool)
	for raw != "" {
		name, rest, eq := strings.Cut(raw, "=")
		if !eq || name == "" || seen[name] || len(attributes) >= 16 ||
			strings.IndexFunc(name, func(r rune) bool { return !(r >= 'A' && r <= 'Z' || r >= '0' && r <= '9' || r == '-') }) >= 0 {
			return nil, false
		}
		seen[name] = true
		field := hlsAttribute{name: name}
		if field.quoted = strings.HasPrefix(rest, "\""); field.quoted {
			var closeQuote bool
			field.value, rest, closeQuote = strings.Cut(rest[1:], "\"")
			if !closeQuote {
				return nil, false
			}
		} else {
			field.value, _, _ = strings.Cut(rest, ",")
			rest = rest[len(field.value):]
		}
		if field.value == "" || strings.Contains(field.value, "{$") {
			return nil, false
		}
		attributes = append(attributes, field)
		if rest == "" {
			return attributes, true
		}
		if !strings.HasPrefix(rest, ",") || len(rest) == 1 {
			return nil, false
		}
		raw = rest[1:]
	}
	return nil, false
}

func validHLSStart(raw string) bool {
	attrs, ok := parseHLSAttributes(raw)
	if !ok {
		return false
	}
	hasOffset := false
	for _, attr := range attrs {
		if attr.quoted {
			return false
		}
		switch attr.name {
		case "TIME-OFFSET":
			if _, ok := parseHLSDecimal(attr.value, true); !ok {
				return false
			}
			hasOffset = true
		case "PRECISE":
			if attr.value != "YES" && attr.value != "NO" {
				return false
			}
		default:
			return false
		}
	}
	return hasOffset
}

func (a hlsAttribute) safeQuoted(allowed func(rune) bool) bool {
	return a.quoted && len(a.value) <= 256 && strings.IndexFunc(a.value, func(r rune) bool { return !allowed(r) }) < 0
}

func hlsIdentifier(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsNumber(r) || unicode.IsMark(r) || strings.ContainsRune(" _-().,+", r)
}

func hlsCodec(r rune) bool {
	return r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' || strings.ContainsRune(".,-", r)
}

func (s *hlsManifestState) acceptMasterAttributes(tag, raw string) bool {
	attrs, ok := parseHLSAttributes(raw)
	if !ok {
		return false
	}
	if tag == "#EXT-X-STREAM-INF" {
		bandwidth := false
		for _, attr := range attrs {
			if !validHLSVariantAttribute(attr) {
				return false
			}
			if attr.name == "BANDWIDTH" {
				bandwidth = true
			}
			if attr.name == "AUDIO" {
				s.referencedAudio[attr.value] = true
			}
		}
		if !bandwidth {
			return false
		}
		s.pendingVariant = true
		s.manifest.literal(tag + ":" + raw)
		return true
	}

	var group, name, mediaType, defaultValue, autoSelect string
	var resource hlsResource
	for _, attr := range attrs {
		switch attr.name {
		case "TYPE":
			if attr.quoted || attr.value != "AUDIO" {
				return false
			}
			mediaType = attr.value
		case "GROUP-ID", "NAME", "LANGUAGE", "ASSOC-LANGUAGE", "CHARACTERISTICS":
			if !attr.safeQuoted(hlsIdentifier) {
				return false
			}
			if attr.name == "GROUP-ID" {
				group = attr.value
			}
			if attr.name == "NAME" {
				name = attr.value
			}
		case "DEFAULT", "AUTOSELECT":
			if attr.quoted || (attr.value != "YES" && attr.value != "NO") {
				return false
			}
			if attr.name == "DEFAULT" {
				defaultValue = attr.value
			} else {
				autoSelect = attr.value
			}
		case "CHANNELS":
			number, valid := parseCatalogOffset(attr.value)
			if !attr.quoted || !valid || number == 0 {
				return false
			}
		case "URI":
			resource, ok = resolveHLSResource(s.base, attr.value, true)
			if !attr.quoted || !ok {
				return false
			}
		default:
			return false
		}
	}
	if group == "" || name == "" || mediaType == "" || (defaultValue == "YES" && autoSelect == "NO") {
		return false
	}
	s.audioGroups[group] = true
	s.appendURIAttribute(tag, attrs, resource)
	return true
}

func validHLSVariantAttribute(attr hlsAttribute) bool {
	switch attr.name {
	case "BANDWIDTH", "AVERAGE-BANDWIDTH":
		number, ok := parseCatalogOffset(attr.value)
		return !attr.quoted && ok && number > 0
	case "RESOLUTION":
		width, height, split := strings.Cut(attr.value, "x")
		w, validWidth := parseCatalogOffset(width)
		h, validHeight := parseCatalogOffset(height)
		return !attr.quoted && split && validWidth && validHeight && w > 0 && h > 0
	case "FRAME-RATE":
		number, ok := parseHLSDecimal(attr.value, false)
		return !attr.quoted && ok && number > 0
	case "CODECS":
		return attr.safeQuoted(hlsCodec)
	case "AUDIO":
		return attr.safeQuoted(hlsIdentifier)
	case "HDCP-LEVEL", "CLOSED-CAPTIONS":
		return !attr.quoted && attr.value == "NONE"
	default:
		return false
	}
}

func (s *hlsManifestState) acceptMap(raw string) bool {
	attrs, ok := parseHLSAttributes(raw)
	if !ok {
		return false
	}
	var resource hlsResource
	for _, attr := range attrs {
		if !attr.quoted {
			return false
		}
		switch attr.name {
		case "URI":
			resource, ok = resolveHLSResource(s.base, attr.value, false)
			if !ok {
				return false
			}
		case "BYTERANGE":
			byteRange, ok := parseHLSRange(attr.value)
			if !ok || !byteRange.explicit {
				return false
			}
		default:
			return false
		}
	}
	if resource.url == "" {
		return false
	}
	s.appendURIAttribute("#EXT-X-MAP", attrs, resource)
	return true
}

func (s *hlsManifestState) appendURIAttribute(tag string, attrs []hlsAttribute, resource hlsResource) {
	prefix := tag + ":"
	for _, attr := range attrs {
		if attr.name == "URI" {
			continue
		}
		if prefix != tag+":" {
			prefix += ","
		}
		value := attr.value
		if attr.quoted {
			value = "\"" + value + "\""
		}
		prefix += attr.name + "=" + value
	}
	if resource.url == "" {
		s.manifest.literal(prefix)
		return
	}
	if prefix != tag+":" {
		prefix += ","
	}
	s.manifest.reference(prefix+`URI="`, "\"", resource)
}
