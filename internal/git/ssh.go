package git

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func (g *gitManager) ValidateSSHKey(path string) error {
	if path == "" {
		return fmt.Errorf("SSH key path is empty")
	}

	cleanPath := filepath.Clean(path)
	if strings.Contains(cleanPath, "..") {
		return fmt.Errorf("invalid SSH key path: %s", path)
	}

	info, err := os.Stat(path)
	if os.IsNotExist(err) {
		return fmt.Errorf("SSH key does not exist: %s", path)
	}
	if err != nil {
		return fmt.Errorf("failed to stat SSH key: %w", err)
	}

	if info.IsDir() {
		return fmt.Errorf("SSH key path is a directory: %s", path)
	}

	if info.Mode().Perm()&0077 != 0 {
		return fmt.Errorf("SSH key has too open permissions: %s", path)
	}

	cmd := exec.Command("ssh-keygen", "-l", "-f", path)
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("invalid SSH key format: %w", err)
	}

	return nil
}
