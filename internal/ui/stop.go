package ui

import (
	"context"
	"os"
	"os/signal"
)

func NotifyStop(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return signal.NotifyContext(parent, StopSignals()...)
}

// HeadlessStopSignals includes SIGINT so headless -p matches documented ctrl+c cancel.
func HeadlessStopSignals() []os.Signal {
	return append([]os.Signal{os.Interrupt}, StopSignals()...)
}

func NotifyStopHeadless(parent context.Context) (context.Context, context.CancelFunc) {
	if parent == nil {
		parent = context.Background()
	}
	return signal.NotifyContext(parent, HeadlessStopSignals()...)
}

func CloseStdinOnDone(ctx context.Context) {
	if ctx == nil {
		return
	}
	go func() {
		<-ctx.Done()
		_ = os.Stdin.Close()
	}()
}
