package cmd

import (
	"context"
	"errors"
	"fmt"

	"github.com/mars/marspi-cli/internal/session"
	"github.com/mars/marspi-core/agentctx"
)

func (a *App) newSession(path, systemPrompt string) (*agentctx.Manager, error) {
	history := agentctx.New(a.cfg.MaxContext, a.provider, a.registry.Schemas(), a.console)
	messages, err := (session.File{Path: path}).Load()
	if err != nil {
		if !errors.Is(err, session.ErrCorrupt) {
			return nil, err
		}
		a.console.Warning(err.Error())
	}
	history.Messages = messages
	if history.Len() == 0 {
		history.AppendSystem(systemPrompt)
	}
	return history, nil
}

func (a *App) runSession(runCtx context.Context, history *agentctx.Manager, path, input string) error {
	_, runErr := a.runner.Run(runCtx, history, input)
	history.ClearRuntimeInjections()
	if saveErr := (session.File{Path: path}).Save(history.Messages); saveErr != nil {
		return errors.Join(runErr, fmt.Errorf("save session: %w", saveErr))
	}
	return runErr
}
