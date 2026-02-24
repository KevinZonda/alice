package connector

import "context"

type AutomationRunner interface {
	Run(ctx context.Context)
}

func (a *App) SetAutomationRunner(runner AutomationRunner) {
	if a == nil {
		return
	}
	a.automationMu.Lock()
	a.automationRunner = runner
	a.automationMu.Unlock()
}

func (a *App) startBackgroundAutomation(ctx context.Context) {
	if a == nil {
		return
	}
	a.automationMu.Lock()
	runner := a.automationRunner
	a.automationMu.Unlock()
	if runner != nil {
		go runner.Run(ctx)
		return
	}
	if a.processor != nil {
		go a.idleSummaryLoop(ctx)
		go a.sessionStateFlushLoop(ctx)
	}
}
