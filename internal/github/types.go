package github

import "time"

type CachedOrg struct {
	Org      string    `json:"org"`
	Repos    []Repo    `json:"repos"`
	CachedAt time.Time `json:"cached_at"`
	TTL      string    `json:"ttl"`
}
