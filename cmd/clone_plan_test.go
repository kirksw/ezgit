package cmd

import "testing"

func TestDefaultCLICloneWorktreePlanOmitsReview(t *testing.T) {
	plan := defaultCLICloneWorktreePlan()
	if !plan.CreateDefault {
		t.Fatal("default CLI clone plan must create the default branch worktree")
	}
	if plan.CreateReview {
		t.Fatal("default CLI clone plan must not create the review worktree")
	}
	if len(plan.Custom) != 0 {
		t.Fatalf("default CLI clone plan custom worktrees = %v, want none", plan.Custom)
	}
}

func TestCLICloneWorktreePlanForReviewCreatesRequestedReview(t *testing.T) {
	plan := cliCloneWorktreePlanFor("main", "review")
	if !plan.CreateDefault || !plan.CreateReview {
		t.Fatalf("review plan = %+v, want default and review worktrees", plan)
	}
	if len(plan.Custom) != 0 {
		t.Fatalf("review plan custom worktrees = %v, want none", plan.Custom)
	}
}

func TestCLICloneWorktreePlanForFeatureCreatesCustomWorktree(t *testing.T) {
	plan := cliCloneWorktreePlanFor("main", "feature-x")
	if !plan.CreateDefault || plan.CreateReview {
		t.Fatalf("feature plan = %+v, want default without review", plan)
	}
	if len(plan.Custom) != 1 || plan.Custom[0].Name != "feature-x" || plan.Custom[0].BaseBranch != "main" {
		t.Fatalf("feature plan custom worktrees = %+v", plan.Custom)
	}
}

func TestRunWithDefaultCLIClonePlanBypassesWorktreeSelection(t *testing.T) {
	originalPlan := forcedClonePlan
	defer func() {
		forcedClonePlan = originalPlan
	}()
	forcedClonePlan = nil

	if err := runWithDefaultCLIClonePlan(func() error {
		if forcedClonePlan == nil {
			t.Fatal("CLI clone plan was not forced")
		}
		if !forcedClonePlan.CreateDefault || forcedClonePlan.CreateReview || len(forcedClonePlan.Custom) != 0 {
			t.Fatalf("forced CLI clone plan = %+v, want default worktree only", *forcedClonePlan)
		}
		return nil
	}); err != nil {
		t.Fatalf("runWithDefaultCLIClonePlan() error = %v", err)
	}

	if forcedClonePlan != nil {
		t.Fatalf("forced clone plan was not restored: %+v", *forcedClonePlan)
	}
}
