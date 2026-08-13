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
