package git

import (
	"testing"
)

func TestParseRepoURL(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    string
		wantErr bool
	}{
		{
			name:    "owner/repo format",
			input:   "facebook/react",
			want:    "git@github.com:facebook/react.git",
			wantErr: false,
		},
		{
			name:    "SSH URL",
			input:   "git@github.com:facebook/react.git",
			want:    "git@github.com:facebook/react.git",
			wantErr: false,
		},
		{
			name:    "SSH URL without .git",
			input:   "git@github.com:facebook/react",
			want:    "git@github.com:facebook/react.git",
			wantErr: false,
		},
		{
			name:    "HTTPS URL",
			input:   "https://github.com/facebook/react",
			want:    "https://github.com/facebook/react.git",
			wantErr: false,
		},
		{
			name:    "invalid format",
			input:   "invalid",
			want:    "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseRepoURL(tt.input)
			if (err != nil) != tt.wantErr {
				t.Errorf("ParseRepoURL() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("ParseRepoURL() = %v, want %v", got, tt.want)
			}
		})
	}
}
