package git

import (
	"fmt"
	"os/exec"
	"strings"
)

func (g *gitManager) Clone(url, path string, opts CloneOptions) error {
	var args []string

	if opts.Bare {
		args = append(args, "clone", "--bare")
	} else {
		args = append(args, "clone")
	}

	if opts.Branch != "" {
		args = append(args, "--branch", opts.Branch)
	}

	if opts.Depth > 0 {
		args = append(args, "--depth", fmt.Sprintf("%d", opts.Depth))
	}

	if opts.Quiet {
		args = append(args, "--quiet")
	}

	if opts.SSHKeyPath != "" {
		if err := g.ValidateSSHKey(opts.SSHKeyPath); err != nil {
			return fmt.Errorf("invalid SSH key: %w", err)
		}
		args = append(args, "--config", fmt.Sprintf("core.sshCommand=ssh -i %s -o IdentitiesOnly=yes", opts.SSHKeyPath))
	}

	args = append(args, url, path)

	cmd := exec.Command("git", args...)
	output, err := cmd.CombinedOutput()
	if err != nil {
		if len(output) > 0 {
			return fmt.Errorf("git clone failed: %w\n%s", err, string(output))
		}
		return fmt.Errorf("git clone failed: %w", err)
	}
	return nil
}

func ParseRepoURL(input string) (string, error) {
	input = strings.TrimSpace(input)

	if strings.HasPrefix(input, "git@") || strings.HasPrefix(input, "https://") || strings.HasPrefix(input, "http://") {
		if !strings.HasSuffix(input, ".git") {
			input += ".git"
		}
		return input, nil
	}

	if strings.Contains(input, "/") {
		parts := strings.Split(input, "/")
		if len(parts) == 2 {
			return fmt.Sprintf("git@github.com:%s/%s.git", parts[0], parts[1]), nil
		}
	}

	return "", fmt.Errorf("invalid repo format: %s (expected owner/repo or full URL)", input)
}
