package app

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
	"time"

	"github.com/rs/zerolog"
	"go.uber.org/fx"
)

type localTunnelConfig struct {
	Enabled bool
}

type localTunnelRunner struct {
	cmd    *exec.Cmd
	done   chan error
	logger zerolog.Logger
	config localTunnelConfig
}

const localTunnelShutdownGracePeriod = 4 * time.Second

func newLocalTunnelModule() fx.Option {
	return fx.Options(
		fx.Provide(newLocalTunnelRunner),
		fx.Invoke(registerLocalTunnelRunner),
	)
}

func newLocalTunnelRunner(
	config localTunnelConfig,
	logger zerolog.Logger,
) *localTunnelRunner {
	return &localTunnelRunner{
		done:   make(chan error, 1),
		logger: logger.With().Str("component", "local_tunnel").Logger(),
		config: config,
	}
}

func registerLocalTunnelRunner(
	lifecycle fx.Lifecycle,
	runner *localTunnelRunner,
) {
	if !runner.config.Enabled {
		return
	}

	lifecycle.Append(fx.Hook{OnStart: runner.onStart, OnStop: runner.onStop})
}

func (r *localTunnelRunner) onStart(ctx context.Context) error {
	scriptPath, err := resolveLocalTunnelScript()
	if err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, "bash", scriptPath)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	if startErr := cmd.Start(); startErr != nil {
		return fmt.Errorf("start local tunnel: %w", startErr)
	}

	r.cmd = cmd
	go func() {
		r.done <- cmd.Wait()
	}()

	r.logger.Info().
		Str("script", scriptPath).
		Int("pid", cmd.Process.Pid).
		Msg("local tunnel started")

	return nil
}

func (r *localTunnelRunner) onStop(_ context.Context) error {
	if r.cmd == nil || r.cmd.Process == nil {
		return nil
	}

	pid := r.cmd.Process.Pid
	_ = syscall.Kill(-pid, syscall.SIGTERM)

	select {
	case <-time.After(localTunnelShutdownGracePeriod):
		_ = syscall.Kill(-pid, syscall.SIGKILL)
	case <-r.done:
	}

	r.logger.Info().Int("pid", pid).Msg("local tunnel stopped")
	return nil
}

func resolveLocalTunnelScript() (string, error) {
	wd, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("resolve working directory: %w", err)
	}

	candidates := []string{
		filepath.Join(wd, "run-local-tunnel.sh"),
		filepath.Join(wd, "apps", "anchor", "run-local-tunnel.sh"),
	}

	for _, candidate := range candidates {
		if stat, statErr := os.Stat(candidate); statErr == nil && !stat.IsDir() {
			return candidate, nil
		}
	}

	return "", fmt.Errorf("run-local-tunnel.sh not found from %s", wd)
}
