package app

import (
	"net/http"
	"strconv"
	"strings"
	"time"
)

// Use only explicit public freshness. Taking the earlier of the expiry
// calculated from Date and from Age includes time spent on the request.
func catalogCoverFreshUntil(headers http.Header, requestedAt time.Time) time.Time {
	if len(headers.Values("Vary")) != 0 || len(headers.Values("Set-Cookie")) != 0 || len(headers.Values("Pragma")) != 0 {
		return time.Time{}
	}
	var public, hasMaxAge bool
	var maxAge uint64
	for line := range strings.SplitSeq(strings.Join(headers.Values("Cache-Control"), ","), ",") {
		directive, value, hasValue := strings.Cut(strings.TrimSpace(strings.ToLower(line)), "=")
		switch {
		case directive == "public" && !hasValue:
			public = true
		case (directive == "immutable" || directive == "must-revalidate") && !hasValue:
		case directive == "max-age" && hasValue && !hasMaxAge:
			age, err := strconv.ParseUint(value, 10, 32)
			if err != nil {
				return time.Time{}
			}
			maxAge, hasMaxAge = age, true
		default:
			return time.Time{}
		}
	}
	if !public || maxAge == 0 {
		return time.Time{}
	}
	expires := requestedAt.Add(time.Duration(maxAge) * time.Second)
	if values := headers.Values("Age"); len(values) != 0 {
		age, err := strconv.ParseUint(values[0], 10, 32)
		if len(values) != 1 || err != nil || age >= maxAge {
			return time.Time{}
		}
		expires = expires.Add(-time.Duration(age) * time.Second)
	}
	if values := headers.Values("Date"); len(values) != 0 {
		date, err := http.ParseTime(values[0])
		if len(values) != 1 || err != nil {
			return time.Time{}
		}
		if datedExpiry := date.Add(time.Duration(maxAge) * time.Second); datedExpiry.Before(expires) {
			expires = datedExpiry
		}
	}
	return expires
}
