package cache

import (
	"fmt"
	"testing"
	"time"

	"github.com/kirksw/ezgit/internal/github"
)

func BenchmarkGetAllRepos(b *testing.B) {
	benchCases := []struct {
		name        string
		orgCount    int
		reposPerOrg int
	}{
		{name: "small", orgCount: 2, reposPerOrg: 25},
		{name: "medium", orgCount: 5, reposPerOrg: 100},
		{name: "large", orgCount: 10, reposPerOrg: 200},
	}

	for _, benchCase := range benchCases {
		b.Run(benchCase.name, func(b *testing.B) {
			c := &OrgCache{cacheDir: b.TempDir(), ttl: time.Hour}

			now := time.Now()
			for orgIdx := 0; orgIdx < benchCase.orgCount; orgIdx++ {
				orgName := fmt.Sprintf("org-%d", orgIdx)
				repos := make([]github.Repo, 0, benchCase.reposPerOrg)
				for repoIdx := 0; repoIdx < benchCase.reposPerOrg; repoIdx++ {
					repos = append(repos, github.Repo{
						FullName:  fmt.Sprintf("%s/repo-%d", orgName, repoIdx),
						CreatedAt: now.Add(-time.Duration(repoIdx) * time.Minute),
					})
				}
				if err := c.Set(orgName, repos); err != nil {
					b.Fatalf("Set(%s) error = %v", orgName, err)
				}
			}

			b.ResetTimer()
			for i := 0; i < b.N; i++ {
				repos, err := c.GetAllRepos()
				if err != nil {
					b.Fatalf("GetAllRepos() error = %v", err)
				}
				if len(repos) == 0 {
					b.Fatalf("GetAllRepos() returned no repos")
				}
			}
		})
	}
}
