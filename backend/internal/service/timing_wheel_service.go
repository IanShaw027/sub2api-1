package service

import (
	"log"
	"sync"
	"time"
)

// TimingWheelService provides lightweight in-process task scheduling.
//
// It intentionally avoids external scheduling dependencies; for this project's needs we only
// require named timers, recurring jobs, and cancellation.
type TimingWheelService struct {
	mu sync.Mutex

	oneShot   map[string]*time.Timer
	recurring map[string]*recurringTask

	stopped  bool
	stopOnce sync.Once
}

type recurringTask struct {
	interval time.Duration
	fn       func()
	timer    *time.Timer
	canceled bool
}

// NewTimingWheelService creates a new TimingWheelService instance
func NewTimingWheelService() *TimingWheelService {
	return &TimingWheelService{
		oneShot:   make(map[string]*time.Timer),
		recurring: make(map[string]*recurringTask),
	}
}

// Start starts the timing wheel
func (s *TimingWheelService) Start() {
	log.Println("[TimingWheel] Started")
}

// Stop stops the timing wheel
func (s *TimingWheelService) Stop() {
	s.stopOnce.Do(func() {
		s.mu.Lock()
		s.stopped = true

		for name, timer := range s.oneShot {
			if timer != nil {
				timer.Stop()
			}
			delete(s.oneShot, name)
		}

		for name, task := range s.recurring {
			if task != nil {
				task.canceled = true
				if task.timer != nil {
					task.timer.Stop()
				}
			}
			delete(s.recurring, name)
		}
		s.mu.Unlock()
		log.Println("[TimingWheel] Stopped")
	})
}

// Schedule schedules a one-time task
func (s *TimingWheelService) Schedule(name string, delay time.Duration, fn func()) {
	if s == nil || name == "" || fn == nil {
		return
	}
	if delay < 0 {
		delay = 0
	}

	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	if old := s.oneShot[name]; old != nil {
		old.Stop()
	}
	timer := time.AfterFunc(delay, func() {
		fn()
		s.mu.Lock()
		delete(s.oneShot, name)
		s.mu.Unlock()
	})
	s.oneShot[name] = timer
	s.mu.Unlock()
}

// ScheduleRecurring schedules a recurring task
func (s *TimingWheelService) ScheduleRecurring(name string, interval time.Duration, fn func()) {
	if s == nil || name == "" || fn == nil {
		return
	}
	if interval <= 0 {
		return
	}

	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}

	// Cancel any existing timer with the same name (one-shot or recurring).
	if old := s.oneShot[name]; old != nil {
		old.Stop()
		delete(s.oneShot, name)
	}
	if old := s.recurring[name]; old != nil {
		old.canceled = true
		if old.timer != nil {
			old.timer.Stop()
		}
		delete(s.recurring, name)
	}

	task := &recurringTask{interval: interval, fn: fn}
	s.recurring[name] = task
	s.mu.Unlock()

	s.armRecurring(name, interval)
}

func (s *TimingWheelService) armRecurring(name string, delay time.Duration) {
	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	task := s.recurring[name]
	if task == nil || task.canceled {
		s.mu.Unlock()
		return
	}
	task.timer = time.AfterFunc(delay, func() {
		s.runRecurring(name)
	})
	s.mu.Unlock()
}

func (s *TimingWheelService) runRecurring(name string) {
	if s == nil || name == "" {
		return
	}

	var (
		fn       func()
		interval time.Duration
	)

	s.mu.Lock()
	if s.stopped {
		s.mu.Unlock()
		return
	}
	task := s.recurring[name]
	if task == nil || task.canceled {
		s.mu.Unlock()
		return
	}
	fn = task.fn
	interval = task.interval
	s.mu.Unlock()

	fn()

	s.armRecurring(name, interval)
}

// Cancel cancels a scheduled task
func (s *TimingWheelService) Cancel(name string) {
	if s == nil || name == "" {
		return
	}

	s.mu.Lock()
	if t := s.oneShot[name]; t != nil {
		t.Stop()
		delete(s.oneShot, name)
	}
	if t := s.recurring[name]; t != nil {
		t.canceled = true
		if t.timer != nil {
			t.timer.Stop()
		}
		delete(s.recurring, name)
	}
	s.mu.Unlock()
}
