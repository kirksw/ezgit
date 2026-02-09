package cmd

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/kirksw/ezgit/internal/ui"
	"github.com/spf13/cobra"
)

var connectCmd = &cobra.Command{
	Use:   "connect [session]",
	Short: "Connect to a tmux session",
	Args:  cobra.MaximumNArgs(1),
	RunE:  runConnect,
}

func init() {
	rootCmd.AddCommand(connectCmd)
}

func runConnect(cmd *cobra.Command, args []string) error {
	session := ""
	if len(args) > 0 {
		session = strings.TrimSpace(args[0])
		if session == "" {
			return fmt.Errorf("session name cannot be empty")
		}
	} else {
		sessions, err := listTmuxSessions()
		if err != nil {
			return err
		}
		if len(sessions) == 0 {
			return fmt.Errorf("no tmux sessions found")
		}

		if !isInteractiveStdin() {
			return fmt.Errorf("session name required when not running interactively")
		}

		selected, cancelled, err := ui.RunTmuxSessionSearch(sessions)
		if err != nil {
			return fmt.Errorf("failed to select tmux session: %w", err)
		}
		if cancelled {
			return nil
		}
		session = strings.TrimSpace(selected)
		if session == "" {
			return nil
		}
	}

	return attachTmuxSession(session)
}

func listTmuxSessions() ([]string, error) {
	cmd := exec.Command("tmux", "list-sessions", "-F", "#{session_name}")
	output, err := cmd.CombinedOutput()
	if err != nil {
		var execErr *exec.Error
		if errors.As(err, &execErr) && errors.Is(execErr.Err, exec.ErrNotFound) {
			return nil, fmt.Errorf("tmux is not installed or not on PATH")
		}

		errText := strings.TrimSpace(string(output))
		if strings.Contains(errText, "no server running") {
			return []string{}, nil
		}
		if errText != "" {
			return nil, fmt.Errorf("failed to list tmux sessions: %s", errText)
		}
		return nil, fmt.Errorf("failed to list tmux sessions: %w", err)
	}

	return parseTmuxSessionList(string(output)), nil
}

func parseTmuxSessionList(output string) []string {
	lines := strings.Split(output, "\n")
	sessions := make([]string, 0, len(lines))
	seen := make(map[string]struct{}, len(lines))
	for _, line := range lines {
		session := strings.TrimSpace(line)
		if session == "" {
			continue
		}
		if _, ok := seen[session]; ok {
			continue
		}
		seen[session] = struct{}{}
		sessions = append(sessions, session)
	}
	return sessions
}

func attachTmuxSession(session string) error {
	cmd := exec.Command("tmux", "attach-session", "-t", session)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("failed to connect to tmux session %q: %w", session, err)
	}
	return nil
}
