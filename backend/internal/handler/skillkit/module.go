package skillkit

import (
	"context"
	"database/sql"
	"fmt"
	"sync"

	"github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type Module struct {
	Config            *config.Config
	EntClient         *ent.Client
	DB                *sql.DB
	DomainRepo        repository.AISkillRepository
	Queries           *Queries
	SkillService      *service.AISkillService
	VersionService    *service.AISkillVersionService
	ReviewService     *service.AISkillReviewService
	RunService        *service.AISkillRunService
	SettlementService *service.AISkillSettlementService
}

var (
	defaultModuleOnce sync.Once
	defaultModule     *Module
	defaultModuleErr  error
	defaultModuleMu   sync.RWMutex
)

func Default() (*Module, error) {
	defaultModuleMu.RLock()
	if defaultModule != nil || defaultModuleErr != nil {
		module, err := defaultModule, defaultModuleErr
		defaultModuleMu.RUnlock()
		return module, err
	}
	defaultModuleMu.RUnlock()

	defaultModuleOnce.Do(func() {
		module, err := buildDefaultModule()
		defaultModuleMu.Lock()
		defaultModule = module
		defaultModuleErr = err
		defaultModuleMu.Unlock()
	})

	defaultModuleMu.RLock()
	module, err := defaultModule, defaultModuleErr
	defaultModuleMu.RUnlock()
	return module, err
}

func NewModule(
	cfg *config.Config,
	entClient *ent.Client,
	db *sql.DB,
	domainRepo repository.AISkillRepository,
	skillService *service.AISkillService,
	versionService *service.AISkillVersionService,
	reviewService *service.AISkillReviewService,
	settlementService *service.AISkillSettlementService,
	runService *service.AISkillRunService,
) *Module {
	return &Module{
		Config:            cfg,
		EntClient:         entClient,
		DB:                db,
		DomainRepo:        domainRepo,
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
	defaultModuleErr = nil
}

func buildDefaultModule() (*Module, error) {
	cfg, err := config.ProvideConfig()
	if err != nil {
		return nil, fmt.Errorf("load skill module config: %w", err)
	}

	entClient, err := repository.ProvideEnt(cfg)
	if err != nil {
		return nil, fmt.Errorf("init skill module ent client: %w", err)
	}
	db, err := repository.ProvideSQLDB(entClient)
	if err != nil {
		return nil, fmt.Errorf("init skill module sql db: %w", err)
	}

	domainRepo := repository.NewAISkillRepository(entClient, db)
	userRepo := repository.NewUserRepository(entClient, db)
	affiliateRepo := repository.NewAffiliateRepository(entClient, db)

	adapter := &serviceRepoAdapter{
		domainRepo: domainRepo,
		db:         db,
	}
	adapter.balanceCharger = &balanceChargerAdapter{userRepo: userRepo}
	adapter.creatorCreditor = service.NewAffiliateService(affiliateRepo, nil, nil, nil)

	settlementService := service.NewAISkillSettlementService(adapter, adapter.balanceCharger, adapter.creatorCreditor)
	versionService := service.NewAISkillVersionService(adapter, adapter, adapter)
	reviewService := service.NewAISkillReviewService(adapter, adapter, adapter)
	runService := service.NewAISkillRunService(adapter, adapter, adapter, settlementService, service.ProvideAISkillRuntimeGateway(nil))

	return NewModule(
		cfg,
		entClient,
		db,
		domainRepo,
		service.NewAISkillService(adapter),
		versionService,
		reviewService,
		settlementService,
		runService,
	), nil
}

type balanceChargerUserRepository interface {
	DeductBalance(ctx context.Context, id int64, amount float64) error
	GetByID(ctx context.Context, id int64) (*service.User, error)
}

type balanceRefundUserRepository interface {
	AddBalanceWithoutRecharge(ctx context.Context, id int64, amount float64) error
}

type balanceChargerAdapter struct {
	userRepo balanceChargerUserRepository
}

func (a *balanceChargerAdapter) ChargeUserBalance(ctx context.Context, input service.AISkillBalanceChargeInput) (*service.AISkillBalanceChargeResult, error) {
	if a == nil || a.userRepo == nil {
		return nil, service.ErrAISkillBalanceServiceUnavailable
	}
	if err := a.userRepo.DeductBalance(ctx, input.UserID, input.Amount); err != nil {
		return nil, err
	}
	user, err := a.userRepo.GetByID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	return &service.AISkillBalanceChargeResult{
		ChargedAmount: input.Amount,
		BalanceAfter:  user.Balance,
	}, nil
}

func (a *balanceChargerAdapter) RefundUserBalance(ctx context.Context, input service.AISkillBalanceRefundInput) (*service.AISkillBalanceRefundResult, error) {
	if a == nil || a.userRepo == nil {
		return nil, service.ErrAISkillBalanceServiceUnavailable
	}
	refundRepo, ok := a.userRepo.(balanceRefundUserRepository)
	if !ok {
		return nil, service.ErrAISkillBalanceServiceUnavailable
	}
	if err := refundRepo.AddBalanceWithoutRecharge(ctx, input.UserID, input.Amount); err != nil {
		return nil, err
	}
	user, err := a.userRepo.GetByID(ctx, input.UserID)
	if err != nil {
		return nil, err
	}
	return &service.AISkillBalanceRefundResult{
		RefundedAmount: input.Amount,
		BalanceAfter:   user.Balance,
	}, nil
}
