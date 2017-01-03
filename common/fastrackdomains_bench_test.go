package common

import (
	"net/http"
	"testing"
)

func BenchmarkFastrackDomains(b *testing.B) {
	wl := NewRawFasttrackDomains("whitelisted1.com,whitelisted2.com,whitelisted3.com")
	req, _ := http.NewRequest("GET", "https://whitelisted.not/page.html", nil)
	for i := 0; i < b.N; i++ {
		wl.Whitelisted(req)
	}
}