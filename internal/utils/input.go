package utils

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

func SelectBranches(branches []string) ([]string, error) {
	if len(branches) == 0 {
		return nil, fmt.Errorf("no branches available")
	}

	fmt.Println("Found branches:")
	for _, branch := range branches {
		fmt.Printf("  [ ] %s\n", branch)
	}

	fmt.Println("\nWhich branches to create worktrees for?")
	fmt.Println("Enter branch names separated by commas (e.g., main,develop):")

	reader := bufio.NewReader(os.Stdin)
	input, err := reader.ReadString('\n')
	if err != nil {
		return nil, fmt.Errorf("failed to read input: %w", err)
	}

	input = strings.TrimSpace(input)
	if input == "" {
		return nil, fmt.Errorf("no branches selected")
	}

	selected := strings.Split(input, ",")
	for i := range selected {
		selected[i] = strings.TrimSpace(selected[i])
	}

	return selected, nil
}

func ValidateBranches(available, selected []string) error {
	availableMap := make(map[string]bool)
	for _, branch := range available {
		availableMap[branch] = true
	}

	for _, branch := range selected {
		if !availableMap[branch] {
			return fmt.Errorf("branch not found: %s", branch)
		}
	}

	return nil
}
