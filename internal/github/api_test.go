package github

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchReposCreatedAfterStopsAfterThreshold(t *testing.T) {
	cutoff := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
	newer := cutoff.Add(24 * time.Hour)
	older := cutoff.Add(-24 * time.Hour)

	requests := 0
	var server *httptest.Server
	server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		requests++
		switch r.URL.Query().Get("page") {
		case "1":
			w.Header().Set("Content-Type", "application/json")
			w.Header().Set("Link", fmt.Sprintf(`<%s/repos?page=2>; rel="next"`, server.URL))
			_, _ = w.Write([]byte(fmt.Sprintf(`[
				{"full_name":"acme/new","created_at":"%s"},
				{"full_name":"acme/old","created_at":"%s"}
			]`, newer.Format(time.RFC3339), older.Format(time.RFC3339))))
		case "2":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`[{"full_name":"acme/older","created_at":"2020-01-01T00:00:00Z"}]`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := NewClient("token")
	repos, err := client.fetchReposCreatedAfter(server.URL+"/repos?page=1", cutoff)
	if err != nil {
		t.Fatalf("fetchReposCreatedAfter() error = %v", err)
	}

	if len(repos) != 1 {
		t.Fatalf("len(repos) = %d, want 1", len(repos))
	}
	if repos[0].FullName != "acme/new" {
		t.Fatalf("repos[0].FullName = %s, want acme/new", repos[0].FullName)
	}
	if requests != 1 {
		t.Fatalf("requests = %d, want 1 (should stop after threshold on page 1)", requests)
	}
}
