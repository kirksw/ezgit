package cmd

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunSkillPrintsAgentSkill(t *testing.T) {
	var out bytes.Buffer
	skillCmd.SetOut(&out)
	t.Cleanup(func() { skillCmd.SetOut(nil) })

	if err := runSkill(skillCmd, nil); err != nil {
		t.Fatalf("runSkill() error = %v", err)
	}

	printed := out.String()
	for _, want := range []string{"---\nname: ezgit\n", "ezgit wt add owner/repo feature-name", "ezgit wt prune owner/repo"} {
		if !strings.Contains(printed, want) {
			t.Fatalf("skill output does not contain %q", want)
		}
	}
}
