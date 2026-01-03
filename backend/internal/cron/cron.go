package cron

import (
	"context"
	"log"

	"github.com/robfig/cron/v3"
)

type Manager struct {
	cron *cron.Cron
	ctx  context.Context
}

func NewManager(ctx context.Context) *Manager {
	return &Manager{
		cron: cron.New(),
		ctx:  ctx,
	}
}

func (m *Manager) AddJob(spec string, job func()) error {
	_, err := m.cron.AddFunc(spec, func() {
		defer func() {
			if r := recover(); r != nil {
				log.Printf("[CRON] Job panic recovered: %v", r)
			}
		}()
		job()
	})
	return err
}

func (m *Manager) Start() {
	m.cron.Start()
	log.Println("[CRON] Manager started")
}

func (m *Manager) Stop() context.Context {
	stopCtx := m.cron.Stop()
	log.Println("[CRON] Manager stopped")
	return stopCtx
}
