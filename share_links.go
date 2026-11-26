package main

import (
	"context"
	"regexp"
	"strconv"
	"strings"
	"sync"

	"jindi-drama/internal/provider/mock"
)

var shareLinkPattern = regexp.MustCompile(`^jindi://(drama|watch)/(d[1-9][0-9]{0,5})(?:/([1-9][0-9]{0,3}))?$`)

var cmsDetailSharePattern = regexp.MustCompile(`^jindi://catalog/apple-cms:[0-9a-f]{32}:[0-9]{1,30}$`)

type shareLinks struct {
	catalog *mock.Provider
	mu      sync.Mutex
	pending string
	reveal  bool
	ready   bool
	deliver func(string)
}

func newShareLinks() *shareLinks { return &shareLinks{catalog: mock.New()} }

func (s *shareLinks) open(raw string) {
	if !cmsDetailSharePattern.MatchString(raw) {
		match := shareLinkPattern.FindStringSubmatch(raw)
		if len(match) == 0 {
			return
		}
		drama, err := s.catalog.Drama(context.Background(), match[2])
		episode, _ := strconv.Atoi(match[3])
		if err != nil || (match[1] == "drama" && episode != 0) ||
			(match[1] == "watch" && (episode == 0 || episode > drama.UpdatedTo)) {
			return
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	s.pending = "/" + strings.TrimPrefix(raw, "jindi://")
	s.flush()
}

func (s *shareLinks) openArgs(args []string) {
	if len(args) == 1 {
		s.open(args[0])
	}
}

func (s *shareLinks) revealWindow() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.reveal = true
	s.flush()
}

func (s *shareLinks) start(deliver func(string)) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.deliver = deliver
	s.flush()
}

func (s *shareLinks) frontendReady() {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.ready = true
	s.flush()
}

func (s *shareLinks) flush() {
	if s.ready && s.deliver != nil && (s.pending != "" || s.reveal) {
		route := s.pending
		s.pending = ""
		s.reveal = false
		s.deliver(route)
	}
}
