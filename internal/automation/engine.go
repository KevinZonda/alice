package automation

import (
	"context"
	"errors"
	"log"
	"strings"
	"sync"
	"time"
)

type TextSender interface {
	SendText(ctx context.Context, receiveIDType, receiveID, text string) error
}

type SystemTaskFunc func(ctx context.Context)

type Engine struct {
	store       *Store
	sender      TextSender
	tick        time.Duration
	maxClaim    int
	now         func() time.Time
	systemsMu   sync.Mutex
	systemTasks map[string]*systemTaskRuntime
}

type systemTaskRuntime struct {
	name     string
	interval time.Duration
	run      SystemTaskFunc
	nextRun  time.Time
	running  bool
}

func NewEngine(store *Store, sender TextSender) *Engine {
	return &Engine{
		store:       store,
		sender:      sender,
		tick:        time.Second,
		maxClaim:    32,
		now:         time.Now,
		systemTasks: make(map[string]*systemTaskRuntime),
	}
}

func (e *Engine) RegisterSystemTask(name string, interval time.Duration, run SystemTaskFunc) error {
	if e == nil {
		return errors.New("engine is nil")
	}
	name = strings.TrimSpace(name)
	if name == "" {
		return errors.New("system task name is empty")
	}
	if interval <= 0 {
		return errors.New("system task interval must be > 0")
	}
	if run == nil {
		return errors.New("system task function is nil")
	}

	e.systemsMu.Lock()
	defer e.systemsMu.Unlock()
	if _, exists := e.systemTasks[name]; exists {
		return errors.New("system task already exists")
	}
	firstRun := e.nowTime()
	e.systemTasks[name] = &systemTaskRuntime{
		name:     name,
		interval: interval,
		run:      run,
		nextRun:  firstRun,
	}
	return nil
}

func (e *Engine) Run(ctx context.Context) {
	if e == nil {
		return
	}
	ticker := time.NewTicker(e.tickDuration())
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			now := e.nowTime()
			e.runSystemTasks(ctx, now)
			e.runUserTasks(ctx, now)
		}
	}
}

func (e *Engine) runSystemTasks(ctx context.Context, now time.Time) {
	e.systemsMu.Lock()
	due := make([]*systemTaskRuntime, 0, len(e.systemTasks))
	for _, task := range e.systemTasks {
		if task == nil || task.run == nil || task.running {
			continue
		}
		if task.nextRun.After(now) {
			continue
		}
		task.running = true
		task.nextRun = now.Add(task.interval)
		due = append(due, task)
	}
	e.systemsMu.Unlock()

	for _, task := range due {
		task := task
		go func() {
			defer e.finishSystemTask(task.name)
			defer func() {
				if recovered := recover(); recovered != nil {
					log.Printf("automation system task panic name=%s err=%v", task.name, recovered)
				}
			}()
			task.run(ctx)
		}()
	}
}

func (e *Engine) finishSystemTask(name string) {
	name = strings.TrimSpace(name)
	if name == "" {
		return
	}
	e.systemsMu.Lock()
	defer e.systemsMu.Unlock()
	task, ok := e.systemTasks[name]
	if !ok || task == nil {
		return
	}
	task.running = false
}

func (e *Engine) runUserTasks(ctx context.Context, now time.Time) {
	if e.store == nil || e.sender == nil {
		return
	}
	claimed, err := e.store.ClaimDueTasks(now, e.maxClaim)
	if err != nil {
		log.Printf("claim automation tasks failed: %v", err)
		return
	}
	for _, task := range claimed {
		task := task
		go e.runUserTask(ctx, task)
	}
}

func (e *Engine) runUserTask(ctx context.Context, task Task) {
	runCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	err := e.executeUserTask(runCtx, task)
	if err != nil {
		log.Printf("run automation task failed id=%s scope=%s:%s err=%v", task.ID, task.Scope.Kind, task.Scope.ID, err)
	}
	if e.store != nil {
		if recordErr := e.store.RecordTaskResult(task.ID, e.nowTime(), err); recordErr != nil {
			log.Printf("record automation result failed id=%s err=%v", task.ID, recordErr)
		}
	}
}

func (e *Engine) executeUserTask(ctx context.Context, task Task) error {
	if e.sender == nil {
		return errors.New("automation sender is nil")
	}
	task = NormalizeTask(task)
	text, err := BuildDispatchText(task.Action)
	if err != nil {
		return err
	}
	if strings.TrimSpace(task.Route.ReceiveIDType) == "" || strings.TrimSpace(task.Route.ReceiveID) == "" {
		return errors.New("task route is incomplete")
	}
	return e.sender.SendText(ctx, task.Route.ReceiveIDType, task.Route.ReceiveID, text)
}

func (e *Engine) tickDuration() time.Duration {
	if e == nil || e.tick <= 0 {
		return time.Second
	}
	return e.tick
}

func (e *Engine) nowTime() time.Time {
	if e == nil || e.now == nil {
		return time.Now().UTC()
	}
	now := e.now()
	if now.IsZero() {
		return time.Now().UTC()
	}
	return now.UTC()
}
