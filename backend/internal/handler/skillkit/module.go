package skillkit

import (
	"database/sql"
	"sync"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type Module struct {
	DomainService     service.AISkillDomainOperations
	Queries           *Queries
	SkillService      *service.AISkillService
	VersionService    *service.AISkillVersionService
	ReviewService     *service.AISkillReviewService
	RunService        *service.AISkillRunService
	SettlementService *service.AISkillSettlementService
}

var (
	defaultModule   *Module
	defaultModuleMu sync.RWMutex
)

func Default() (*Module, error) {
	defaultModuleMu.RLock()
	module := defaultModule
	defaultModuleMu.RUnlock()
	if module == nil {
		return nil, service.ErrAISkillServiceUnavailable
	}
	return module, nil
}

func NewModule(
	db *sql.DB,
	domainService *service.AISkillDomainService,
	skillService *service.AISkillService,
	versionService *service.AISkillVersionService,
	reviewService *service.AISkillReviewService,
	settlementService *service.AISkillSettlementService,
	runService *service.AISkillRunService,
) *Module {
	return &Module{
		DomainService:     domainService,
		Queries:           NewQueries(db),
		SkillService:      skillService,
		VersionService:    versionService,
		ReviewService:     reviewService,
		RunService:        runService,
		SettlementService: settlementService,
	}
}

func SetDefault(module *Module) {
	defaultModuleMu.Lock()
	defer defaultModuleMu.Unlock()
	defaultModule = module
}
