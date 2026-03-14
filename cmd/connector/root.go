package main

import (
	"context"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/cobra"

	"github.com/Alice-space/alice/internal/bootstrap"
	"github.com/Alice-space/alice/internal/config"
	"github.com/Alice-space/alice/internal/logging"
)

func newRootCmd() *cobra.Command {
	configPath := config.DefaultConfigPath
	root := &cobra.Command{
		Use:           "alice-connector",
		Short:         "Run the Alice Feishu connector",
		SilenceUsage:  true,
		SilenceErrors: true,
		Args:          cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConnector(configPath)
		},
	}
	root.PersistentFlags().StringVarP(&configPath, "config", "c", config.DefaultConfigPath, "path to config yaml")
	root.AddCommand(&cobra.Command{
		Use:   "run",
		Short: "Run the connector process",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runConnector(configPath)
		},
	})
	root.AddCommand(newRuntimeCmd())
	return root
}

func runConnector(configPath string) error {
	cfg, err := config.LoadFromFile(configPath)
	if err != nil {
		return err
	}
	if err := logging.Configure(logging.Options{
		Level:      cfg.LogLevel,
		FilePath:   cfg.LogFile,
		MaxSizeMB:  cfg.LogMaxSizeMB,
		MaxBackups: cfg.LogMaxBackups,
		MaxAgeDays: cfg.LogMaxAgeDays,
		Compress:   cfg.LogCompress,
	}); err != nil {
		return err
	}
	logging.Debugf("debug logging enabled log_level=%s config=%s", cfg.LogLevel, configPath)

	llmProvider, err := bootstrap.NewLLMProvider(cfg)
	if err != nil {
		return err
	}

	skillReport, err := bootstrap.EnsureBundledSkillsLinked(cfg.WorkspaceDir)
	if err != nil {
		logging.Warnf("sync bundled skills failed: %v", err)
	} else if skillReport.Discovered > 0 {
		logging.Infof(
			"bundled skills synced codex_home=%s discovered=%d linked=%d updated=%d backed_up=%d unchanged=%d failed=%d",
			skillReport.CodexHome,
			skillReport.Discovered,
			skillReport.Linked,
			skillReport.Updated,
			skillReport.BackedUp,
			skillReport.Unchanged,
			skillReport.Failed,
		)
	}

	runtime, err := bootstrap.BuildConnectorRuntime(cfg, llmProvider)
	if err != nil {
		return err
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	if err := startConfigHotReload(ctx, configPath, runtime); err != nil {
		logging.Warnf("config hot reload disabled: %v", err)
	}

	logging.Infof("feishu-codex connector started (long connection mode)")
	logging.Infof("memory module enabled dir=%s", runtime.MemoryDir)
	logging.Infof("automation engine enabled state_file=%s", runtime.AutomationStatePath)
	if runtime.RuntimeAPI != nil {
		logging.Infof("runtime http api enabled addr=%s", runtime.RuntimeAPIBaseURL)
	}
	if err := runtime.Run(ctx); err != nil {
		return err
	}

	logging.Infof("connector stopped")
	return nil
}

func startConfigHotReload(ctx context.Context, configPath string, runtime *bootstrap.ConnectorRuntime) error {
	if runtime == nil {
		return nil
	}
	absConfigPath, err := filepath.Abs(strings.TrimSpace(configPath))
	if err != nil {
		return err
	}
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		return err
	}
	configDir := filepath.Dir(absConfigPath)
	if err := watcher.Add(configDir); err != nil {
		_ = watcher.Close()
		return err
	}
	logging.Infof("config hot reload enabled path=%s", absConfigPath)

	go func() {
		defer watcher.Close()
		var timerMu sync.Mutex
		var timer *time.Timer
		scheduleReload := func() {
			timerMu.Lock()
			defer timerMu.Unlock()
			if timer != nil {
				timer.Stop()
			}
			timer = time.AfterFunc(300*time.Millisecond, func() {
				reloadConfigFromDisk(absConfigPath, runtime)
			})
		}
		for {
			select {
			case <-ctx.Done():
				return
			case err := <-watcher.Errors:
				if err != nil {
					logging.Warnf("config watcher error: %v", err)
				}
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				if !isConfigFileEvent(event, absConfigPath) {
					continue
				}
				logging.Infof("config change detected path=%s op=%s", absConfigPath, event.Op.String())
				scheduleReload()
			}
		}
	}()
	return nil
}

func isConfigFileEvent(event fsnotify.Event, configPath string) bool {
	changedPath := filepath.Clean(strings.TrimSpace(event.Name))
	targetPath := filepath.Clean(strings.TrimSpace(configPath))
	if changedPath != targetPath {
		return false
	}
	const interestingOps = fsnotify.Write | fsnotify.Create | fsnotify.Rename | fsnotify.Remove | fsnotify.Chmod
	return event.Op&interestingOps != 0
}

func reloadConfigFromDisk(configPath string, runtime *bootstrap.ConnectorRuntime) {
	cfg, err := config.LoadFromFile(configPath)
	if err != nil {
		logging.Warnf("config hot reload skipped: reload config failed path=%s err=%v", configPath, err)
		return
	}
	report, err := runtime.ApplyConfigReload(cfg)
	if err != nil {
		logging.Warnf("config hot reload failed path=%s err=%v", configPath, err)
		return
	}
	if len(report.AppliedFields) > 0 {
		logging.Infof(
			"config hot reload applied path=%s fields=%s",
			configPath,
			strings.Join(report.AppliedFields, ","),
		)
	}
	if len(report.RestartRequiredFields) > 0 {
		logging.Warnf(
			"config hot reload requires restart path=%s fields=%s",
			configPath,
			strings.Join(report.RestartRequiredFields, ","),
		)
	}
}
