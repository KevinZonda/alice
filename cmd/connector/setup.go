package main

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"text/template"

	"github.com/spf13/cobra"

	aliceassets "github.com/Alice-space/alice"
	"github.com/Alice-space/alice/internal/bootstrap"
	"github.com/Alice-space/alice/internal/config"
)

func newSetupCmd() *cobra.Command {
	aliceHome := ""
	serviceName := "alice.service"

	cmd := &cobra.Command{
		Use:   "setup",
		Short: "Initialize Alice runtime directories, config, and OpenCode plugin",
		Long: `Create ALICE_HOME, write initial config, sync bundled skills,
write systemd user unit (Linux), and install the OpenCode delegate plugin.

After setup:
  systemctl --user start alice.service     # Linux
  alice --feishu-websocket                 # manual start`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			aliceHome = strings.TrimSpace(aliceHome)
			if aliceHome == "" {
				aliceHome = config.AliceHomeDir()
			} else {
				aliceHome = config.ResolveAliceHomeDir(aliceHome)
			}
			_ = os.Setenv(config.EnvAliceHome, aliceHome)

			configPath := config.ConfigPathForAliceHome(aliceHome)

			newline := func() { fmt.Fprintln(cmd.OutOrStdout()) }
			info := func(format string, args ...any) {
				fmt.Fprintf(cmd.OutOrStdout(), "[alice setup] "+format+"\n", args...)
			}

			info("alice home: %s", aliceHome)

			// 1. Create directory structure
			for _, dir := range []string{
				filepath.Join(aliceHome, "bin"),
				filepath.Join(aliceHome, "log"),
				filepath.Join(aliceHome, "run"),
				filepath.Join(aliceHome, "prompts"),
			} {
				if err := os.MkdirAll(dir, 0o750); err != nil {
					return fmt.Errorf("create directory %s: %w", dir, err)
				}
			}
			info("directories created")

			// 2. Write config template
			configCreated, err := ensureConfigFileExists(configPath)
			if err != nil {
				return fmt.Errorf("write config: %w", err)
			}
			if configCreated {
				info("wrote initial config: %s", configPath)
			} else {
				info("config already exists: %s", configPath)
			}

			// 3. Sync skills
			skillReport, err := bootstrap.EnsureBundledSkillsLinkedForAliceHome(aliceHome, nil)
			if err != nil {
				info("skill sync failed (Alice will retry on startup): %v", err)
			} else {
				info("skills synced: source=%s linked=%d unchanged=%d",
					skillReport.SourceRoot, skillReport.Linked, skillReport.Unchanged)
			}

			// 4. Write systemd user unit (Linux only)
			if runtime.GOOS == "linux" {
				configHome := os.Getenv("XDG_CONFIG_HOME")
				if configHome == "" {
					configHome = filepath.Join(os.Getenv("HOME"), ".config")
				}
				serviceDir := filepath.Join(configHome, "systemd", "user")
				serviceFile := filepath.Join(serviceDir, serviceName)
				if err := os.MkdirAll(serviceDir, 0o750); err != nil {
					return fmt.Errorf("create systemd dir: %w", err)
				}

				binPath, _ := os.Executable()
				unit, err := renderSystemdUnit(aliceHome, binPath)
				if err != nil {
					return fmt.Errorf("render systemd unit: %w", err)
				}
				if err := os.WriteFile(serviceFile, unit, 0o644); err != nil {
					return fmt.Errorf("write systemd unit: %w", err)
				}
				info("wrote systemd unit: %s", serviceFile)

				// enable-linger
				if lingerOutput, lingerErr := exec.Command("loginctl", "enable-linger").CombinedOutput(); lingerErr == nil {
					info("linger enabled")
				} else {
					info("linger enable skipped (may need sudo): %s", strings.TrimSpace(string(lingerOutput)))
				}

				// daemon-reload
				if reloadOut, reloadErr := exec.Command("systemctl", "--user", "daemon-reload").CombinedOutput(); reloadErr != nil {
					info("systemctl daemon-reload: %s", strings.TrimSpace(string(reloadOut)))
				} else {
					info("systemd daemon-reload done")
				}
			}

			// 5. Write OpenCode plugin
			pluginDir := filepath.Join(os.Getenv("HOME"), ".config", "opencode", "plugins")
			if err := os.MkdirAll(pluginDir, 0o750); err != nil {
				info("plugin dir create failed: %v", err)
			} else {
				pluginPath := filepath.Join(pluginDir, "alice-delegate.js")
				if err := os.WriteFile(pluginPath, aliceassets.OpenCodePluginJS, 0o644); err != nil {
					info("plugin write failed: %v", err)
				} else {
					info("wrote OpenCode plugin: %s", pluginPath)
				}
			}

			newline()
			info("setup complete")

			if runtime.GOOS == "linux" {
				fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
				fmt.Fprintf(cmd.OutOrStdout(), "  1. Edit config: %s\n", configPath)
				fmt.Fprintf(cmd.OutOrStdout(), "  2. Start service: systemctl --user start %s\n", serviceName)
			} else {
				fmt.Fprintln(cmd.OutOrStdout(), "Next steps:")
				fmt.Fprintf(cmd.OutOrStdout(), "  1. Edit config: %s\n", configPath)
				fmt.Fprintf(cmd.OutOrStdout(), "  2. Start: alice --feishu-websocket\n")
			}
			return nil
		},
	}

	cmd.Flags().StringVar(&aliceHome, "alice-home", "", "ALICE_HOME directory (default: ~/.alice)")
	cmd.Flags().StringVar(&serviceName, "service", "alice.service", "systemd service name")
	return cmd
}

func renderSystemdUnit(aliceHome, binPath string) ([]byte, error) {
	tmpl, err := template.New("alice.service").Parse(string(aliceassets.SystemdUnitTmpl))
	if err != nil {
		return nil, err
	}
	var buf bytes.Buffer
	err = tmpl.Execute(&buf, map[string]string{
		"AliceHome": aliceHome,
		"BinPath":   binPath,
	})
	if err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}
