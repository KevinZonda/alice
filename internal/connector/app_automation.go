package connector

import "context"

type AutomationRunner interface {
	Run(ctx context.Context)
}

func (a *App) SetAutomationRunner(runner AutomationRunner) {
	if a == nil {
		return
	}
	a.mu.Lock()
	a.automationRunner = runner
	a.mu.Unlock()
}

func (a *App) startBackgroundAutomation(ctx context.Context) {
	if a == nil {
		return
	}
	a.mu.Lock()
	runner := a.automationRunner
	a.mu.Unlock()
	if runner != nil {
		go runner.Run(ctx)
		return
	}
	if a.processor != nil {
		go a.idleSummaryLoop(ctx)
		go a.sessionStateFlushLoop(ctx)
	}
}
