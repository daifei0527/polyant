package daemon

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/daifei0527/agentwiki/pkg/config"
	"github.com/kardianos/service"
)

type Program struct {
	Config   *config.Config
	StartFn  func() error
	StopFn   func() error
}

func (p *Program) Start(s service.Service) error {
	go p.run()
	return nil
}

func (p *Program) run() {
	if p.StartFn != nil {
		p.StartFn()
	}
}

func (p *Program) Stop(s service.Service) error {
	if p.StopFn != nil {
		return p.StopFn()
	}
	return nil
}

type Daemon struct {
	service service.Service
	config  *config.Config
}

func NewDaemon(cfg *config.Config, startFn, stopFn func() error) (*Daemon, error) {
	svcConfig := &service.Config{
		Name:        "AgentWiki",
		DisplayName: "AgentWiki Distributed Knowledge Base",
		Description: "P2P distributed knowledge base for AI agents",
	}

	prg := &Program{
		Config:  cfg,
		StartFn: startFn,
		StopFn:  stopFn,
	}

	svc, err := service.New(prg, svcConfig)
	if err != nil {
		return nil, fmt.Errorf("create service: %w", err)
	}

	return &Daemon{
		service: svc,
		config:  cfg,
	}, nil
}

func (d *Daemon) Run() error {
	return d.service.Run()
}

func (d *Daemon) Install() error {
	return d.service.Install()
}

func (d *Daemon) Uninstall() error {
	return d.service.Uninstall()
}

func (d *Daemon) Start() error {
	return d.service.Start()
}

func (d *Daemon) Stop() error {
	return d.service.Stop()
}

func (d *Daemon) Status() (string, error) {
	status, err := d.service.Status()
	if err != nil {
		return "unknown", err
	}

	switch status {
	case service.StatusRunning:
		return "running", nil
	case service.StatusStopped:
		return "stopped", nil
	default:
		return "unknown", nil
	}
}

func WaitForSignal(stopFn func()) {
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)
	<-sigCh

	if stopFn != nil {
		stopFn()
	}
}

func RunAsService(cfg *config.Config, startFn, stopFn func() error) error {
	if len(os.Args) > 1 {
		d, err := NewDaemon(cfg, startFn, stopFn)
		if err != nil {
			return err
		}

		cmd := os.Args[1]
		switch cmd {
		case "install":
			return d.Install()
		case "uninstall":
			return d.Uninstall()
		case "start":
			return d.Start()
		case "stop":
			return d.Stop()
		case "status":
			status, err := d.Status()
			if err != nil {
				return err
			}
			fmt.Printf("Service status: %s\n", status)
			return nil
		}
	}

	d, err := NewDaemon(cfg, startFn, stopFn)
	if err != nil {
		return err
	}

	return d.Run()
}
