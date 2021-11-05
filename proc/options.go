package proc

import (
	"context"
	"os"
	"path/filepath"
	"sync"

	"github.com/yubo/golib/configer"
)

type ProcessOptions struct {
	name            string
	ctx             context.Context
	cancel          context.CancelFunc
	description     string
	noloop          bool
	group           bool
	wg              *sync.WaitGroup
	configerOptions []configer.ConfigerOption
}

func newProcessOptions() *ProcessOptions {
	ctx, cancel := context.WithCancel(context.Background())

	return &ProcessOptions{
		name:   filepath.Base(os.Args[0]),
		ctx:    ctx,
		cancel: cancel,
		group:  true,
		wg:     &sync.WaitGroup{},
	}
}

type ProcessOption func(*ProcessOptions)

func WithContext(ctx context.Context) ProcessOption {
	return func(p *ProcessOptions) {
		p.ctx, p.cancel = context.WithCancel(ctx)
	}
}

func WithDescription(description string) ProcessOption {
	return func(p *ProcessOptions) {
		p.description = description
	}
}

func WithoutLoop() ProcessOption {
	return func(p *ProcessOptions) {
		p.noloop = true
	}
}

func WithoutGroup() ProcessOption {
	return func(p *ProcessOptions) {
		p.group = false
	}
}

func WithWaitGroup(wg *sync.WaitGroup) ProcessOption {
	return func(p *ProcessOptions) {
		p.wg = wg
	}
}

func WithConfigOptions(options ...configer.ConfigerOption) ProcessOption {
	return func(p *ProcessOptions) {
		p.configerOptions = options
	}
}
