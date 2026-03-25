package campaignrepo

import (
	"fmt"
	"strings"
	"time"
)

func ResumeWakeTask(root, taskID string, now time.Time, leaseDuration time.Duration, roleDefaults ...CampaignRoleDefaults) (TaskDocument, error) {
	repo, err := Load(root)
	if err != nil {
		return TaskDocument{}, err
	}
	if len(roleDefaults) > 0 {
		repo.ConfigRoleDefaults = roleDefaults[0]
	}
	if now.IsZero() {
		now = time.Now().Local()
	}
	if leaseDuration <= 0 {
		leaseDuration = defaultDispatchLease
	}
	taskID = strings.TrimSpace(taskID)
	for idx := range repo.Tasks {
		task := &repo.Tasks[idx]
		if strings.TrimSpace(task.Frontmatter.TaskID) != taskID {
			continue
		}
		if normalizeTaskStatus(task.Frontmatter.Status) != TaskStatusWaitingExternal {
			return *task, nil
		}
		task.Frontmatter.Status = TaskStatusExecuting
		task.Frontmatter.DispatchState = "wake_resumed"
		task.Frontmatter.OwnerAgent = roleLabel(resolveExecutorRole(repo, *task))
		task.LeaseUntil = now.Add(leaseDuration)
		task.WakeAt = time.Time{}
		task.Frontmatter.WakePrompt = ""
		if task.Frontmatter.ExecutionRound <= 0 {
			task.Frontmatter.ExecutionRound = 1
		}
		if err := persistTaskDocument(&repo, idx); err != nil {
			return TaskDocument{}, err
		}
		return repo.Tasks[idx], nil
	}
	return TaskDocument{}, fmt.Errorf("task %s not found", taskID)
}
