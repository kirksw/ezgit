package utils

import (
	"testing"
)

func TestParseRepoIdentifier(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantOwner string
		wantRepo  string
		wantErr   bool
	}{
		{
			name:      "valid owner/repo",
			input:     "facebook/react",
			wantOwner: "facebook",
			wantRepo:  "react",
			wantErr:   false,
		},
		{
			name:      "SSH URL",
			input:     "git@github.com:facebook/react.git",
			wantOwner: "facebook",
			wantRepo:  "react",
			wantErr:   false,
		},
		{
			name:    "invalid format",
			input:   "invalid",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			owner, repo, err := ParseRepoIdentifier(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRepoIdentifier() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if owner != tt.wantOwner || repo != tt.wantRepo {
				t.Errorf("ParseRepoIdentifier() = (%v, %v), want (%v, %v)", owner, repo, tt.wantOwner, tt.wantRepo)
			}
		})
	}
}

func TestBuildSSHURL(t *testing.T) {
	got := BuildSSHURL("facebook", "react")
	want := "git@github.com:facebook/react.git"
	if got != want {
		t.Errorf("BuildSSHURL() = %v, want %v", got, want)
	}
}
