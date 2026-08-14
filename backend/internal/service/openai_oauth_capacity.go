package service

import (
	"context"
	"fmt"
	"math"
	"sort"
	"strings"
	"time"
)

const (
	openAIOAuthWindow5h            = "5h"
	openAIOAuthWindow7d            = "7d"
	openAIOAuthWarningRemainingPct = 15.0
	openAIOAuthCriticalRemaining   = 8.0
)

var openAIOAuthKnownPlans = []string{"k12", "plus", "team", "pro"}

type OpenAIOAuthRateLimitBuckets struct {
	Total            int `json:"total"`
	UpTo10Minutes    int `json:"up_to_10m"`
	From10To30       int `json:"from_10m_to_30m"`
	From30To1Hour    int `json:"from_30m_to_1h"`
	From1To3Hours    int `json:"from_1h_to_3h"`
	From3To5Hours    int `json:"from_3h_to_5h"`
	From5HoursTo1Day int `json:"from_5h_to_1d"`
	From1To3Days     int `json:"from_1d_to_3d"`
	Over3Days        int `json:"over_3d"`
}

type OpenAIOAuthPlanCount struct {
	PlanType    string `json:"plan_type"`
	Total       int    `json:"total"`
	Schedulable int    `json:"schedulable"`
	Errors      int    `json:"errors"`
	RateLimited int    `json:"rate_limited"`
}

type OpenAIOAuthCapacityWindow struct {
	Window           string     `json:"window"`
	UsedPercent      *float64   `json:"used_percent"`
	RemainingPercent *float64   `json:"remaining_percent"`
	ResetAt          *time.Time `json:"reset_at"`
	BurnRate         float64    `json:"burn_rate"`
	ExhaustsAt       *time.Time `json:"exhausts_at,omitempty"`
	MeasuredAccounts int        `json:"measured_accounts"`
	Alert            string     `json:"alert,omitempty"`
	projectedGap     bool       `json:"-"`
}

type OpenAIOAuthCapacityScope struct {
	GroupID     *int64                      `json:"group_id,omitempty"`
	GroupName   string                      `json:"group_name"`
	Accounts    OpenAIOAuthPlanCount        `json:"accounts"`
	PlanCounts  []OpenAIOAuthPlanCount      `json:"plan_counts"`
	RateLimits  OpenAIOAuthRateLimitBuckets `json:"rate_limits"`
	Windows     []OpenAIOAuthCapacityWindow `json:"windows"`
	Alert       string                      `json:"alert,omitempty"`
	SuggestAdd  int                         `json:"suggest_accounts"`
	ShortfallAt *time.Time                  `json:"first_shortfall_at,omitempty"`
}

type OpenAIOAuthCapacityOverview struct {
	GeneratedAt time.Time                  `json:"generated_at"`
	Total       OpenAIOAuthCapacityScope   `json:"total"`
	Groups      []OpenAIOAuthCapacityScope `json:"groups"`
}

type OpenAIOAuthCapacityService struct {
	accountRepo AccountRepository
	forecast    *CapacityForecastService
}

func NewOpenAIOAuthCapacityService(accountRepo AccountRepository, forecast *CapacityForecastService) *OpenAIOAuthCapacityService {
	return &OpenAIOAuthCapacityService{accountRepo: accountRepo, forecast: forecast}
}

func (s *OpenAIOAuthCapacityService) GetOverview(ctx context.Context, groupID *int64) (*OpenAIOAuthCapacityOverview, error) {
	if s == nil || s.accountRepo == nil {
		return nil, fmt.Errorf("openai oauth capacity service is not configured")
	}
	accounts, err := s.accountRepo.ListAllWithFilters(ctx, PlatformOpenAI, AccountTypeOAuth, "", "", 0, "")
	if err != nil {
		return nil, err
	}
	now := time.Now().UTC()
	oauth := make([]Account, 0, len(accounts))
	for i := range accounts {
		if groupID != nil && !accountInGroup(&accounts[i], *groupID) {
			continue
		}
		oauth = append(oauth, accounts[i])
	}

	overview := &OpenAIOAuthCapacityOverview{
		GeneratedAt: now,
		Total:       buildOpenAIOAuthScope(oauth, nil, "all", now),
		Groups:      []OpenAIOAuthCapacityScope{},
	}
	if groupID == nil {
		overview.Groups = splitOpenAIOAuthGroups(oauth, now)
	}

	if s.forecast != nil {
		ts, tsErr := s.forecast.GetTimeseries(ctx, PlatformOpenAI, groupID, "24h", false)
		if tsErr == nil && ts != nil {
			overview.Total.SuggestAdd = ts.KPIs.SuggestAccounts
			if ts.KPIs.FirstShortfallAt != nil {
				overview.Total.ShortfallAt = ts.KPIs.FirstShortfallAt
				if overview.Total.Alert == "" {
					overview.Total.Alert = "warning"
				}
			}
		}
	}
	return overview, nil
}

func (s *OpenAIOAuthCapacityService) GetTimeseries(ctx context.Context, groupID *int64, rangeKey string, force bool) (*CapacityTimeseries, error) {
	if s == nil || s.forecast == nil {
		return nil, fmt.Errorf("capacity forecast service is not configured")
	}
	if rangeKey == "" {
		rangeKey = "24h"
	}
	return s.forecast.GetTimeseries(ctx, PlatformOpenAI, groupID, rangeKey, force)
}

func accountInGroup(account *Account, groupID int64) bool {
	if account == nil {
		return false
	}
	for _, id := range account.GroupIDs {
		if id == groupID {
			return true
		}
	}
	for _, group := range account.AccountGroups {
		if group.GroupID == groupID {
			return true
		}
	}
	return false
}

func splitOpenAIOAuthGroups(accounts []Account, now time.Time) []OpenAIOAuthCapacityScope {
	grouped := map[int64][]Account{}
	names := map[int64]string{}
	for i := range accounts {
		ids := accounts[i].GroupIDs
		if len(ids) == 0 {
			for _, group := range accounts[i].AccountGroups {
				ids = append(ids, group.GroupID)
				if group.Group != nil {
					names[group.GroupID] = group.Group.Name
				}
			}
		} else {
			for _, group := range accounts[i].AccountGroups {
				if group.Group != nil {
					names[group.GroupID] = group.Group.Name
				}
			}
		}
		if len(ids) == 0 {
			grouped[0] = append(grouped[0], accounts[i])
			continue
		}
		for _, id := range ids {
			grouped[id] = append(grouped[id], accounts[i])
		}
	}
	scopes := make([]OpenAIOAuthCapacityScope, 0, len(grouped))
	for id, rows := range grouped {
		var groupID *int64
		name := "ungrouped"
		if id > 0 {
			groupID = &id
			if names[id] != "" {
				name = names[id]
			}
		}
		scopes = append(scopes, buildOpenAIOAuthScope(rows, groupID, name, now))
	}
	sort.Slice(scopes, func(i, j int) bool {
		return scopes[i].GroupName < scopes[j].GroupName
	})
	return scopes
}

func buildOpenAIOAuthScope(accounts []Account, groupID *int64, name string, now time.Time) OpenAIOAuthCapacityScope {
	scope := OpenAIOAuthCapacityScope{
		GroupID:    groupID,
		GroupName:  name,
		RateLimits: openAIOAuthRateLimitBuckets(accounts, now),
		Windows: []OpenAIOAuthCapacityWindow{
			aggregateOpenAIOAuthWindow(accounts, openAIOAuthWindow5h, now),
			aggregateOpenAIOAuthWindow(accounts, openAIOAuthWindow7d, now),
		},
	}
	plans := map[string]*OpenAIOAuthPlanCount{}
	for i := range accounts {
		account := accounts[i]
		plan := normalizeOpenAIOAuthPlan(account.GetCredential("plan_type"))
		scope.Accounts.Total++
		count := plans[plan]
		if count == nil {
			count = &OpenAIOAuthPlanCount{PlanType: plan}
			plans[plan] = count
		}
		count.Total++
		if account.Schedulable && account.Status != StatusError {
			scope.Accounts.Schedulable++
			count.Schedulable++
		}
		if account.Status == StatusError {
			scope.Accounts.Errors++
			count.Errors++
		}
		if account.RateLimitResetAt != nil && account.RateLimitResetAt.After(now) {
			scope.Accounts.RateLimited++
			count.RateLimited++
		}
	}
	for _, plan := range openAIOAuthKnownPlans {
		if count, ok := plans[plan]; ok {
			scope.PlanCounts = append(scope.PlanCounts, *count)
			delete(plans, plan)
		}
	}
	extras := make([]OpenAIOAuthPlanCount, 0, len(plans))
	for _, count := range plans {
		extras = append(extras, *count)
	}
	sort.Slice(extras, func(i, j int) bool { return extras[i].PlanType < extras[j].PlanType })
	scope.PlanCounts = append(scope.PlanCounts, extras...)
	applyOpenAIOAuthWindowAlerts(scope.Windows)
	for _, window := range scope.Windows {
		if window.Alert == "critical" {
			scope.Alert = "critical"
			break
		}
		if window.Alert == "warning" && scope.Alert == "" {
			scope.Alert = "warning"
		}
	}
	return scope
}

func applyOpenAIOAuthWindowAlerts(windows []OpenAIOAuthCapacityWindow) {
	if len(windows) == 0 {
		return
	}
	shortBurn := windows[0].BurnRate
	longBurn := windows[len(windows)-1].BurnRate
	if windows[0].RemainingPercent == nil {
		shortBurn = longBurn
	}
	if windows[len(windows)-1].RemainingPercent == nil {
		longBurn = shortBurn
	}
	for i := range windows {
		if windows[i].RemainingPercent == nil {
			windows[i].Alert = ""
			continue
		}
		windows[i].Alert = openAIOAuthAlertLevel(*windows[i].RemainingPercent, shortBurn, longBurn, windows[i].projectedGap)
	}
}

func aggregateOpenAIOAuthWindow(accounts []Account, kind string, now time.Time) OpenAIOAuthCapacityWindow {
	var usedSum float64
	var measured int
	var burnSum float64
	var burnN int
	var earliestExhaust *time.Time
	var latestReset *time.Time
	var projectedGap bool
	for i := range accounts {
		window := openAIOAuthWindowFromExtra(accounts[i].Extra, kind, now, 0)
		if window.UsedPercent == nil {
			continue
		}
		measured++
		usedSum += *window.UsedPercent
		if window.BurnRate > 0 {
			burnSum += window.BurnRate
			burnN++
		}
		if window.projectedGap {
			projectedGap = true
		}
		if window.ExhaustsAt != nil && (earliestExhaust == nil || window.ExhaustsAt.Before(*earliestExhaust)) {
			earliestExhaust = window.ExhaustsAt
		}
		if window.ResetAt != nil && (latestReset == nil || window.ResetAt.After(*latestReset)) {
			latestReset = window.ResetAt
		}
	}
	out := OpenAIOAuthCapacityWindow{Window: kind, MeasuredAccounts: measured, ResetAt: latestReset, ExhaustsAt: earliestExhaust, projectedGap: projectedGap}
	if measured == 0 {
		return out
	}
	used := usedSum / float64(measured)
	remaining := math.Max(0, 100-used)
	out.UsedPercent = &used
	out.RemainingPercent = &remaining
	if burnN > 0 {
		out.BurnRate = burnSum / float64(burnN)
	}
	out.Alert = openAIOAuthAlertLevel(remaining, out.BurnRate, out.BurnRate, projectedGap)
	return out
}

func openAIOAuthWindowFromExtra(extra map[string]any, kind string, now time.Time, _ float64) OpenAIOAuthCapacityWindow {
	out := OpenAIOAuthCapacityWindow{Window: kind}
	if extra == nil {
		return out
	}
	usedRaw, ok := extra["codex_"+kind+"_used_percent"]
	if !ok {
		return out
	}
	used := parseExtraFloat64(usedRaw)
	resetRaw, ok := extra["codex_"+kind+"_reset_at"]
	if ok {
		if resetAt, err := parseTime(fmt.Sprint(resetRaw)); err == nil {
			duration := time.Duration(openAICapacityDefaultWindowMinutes[kind]) * time.Minute
			if wm := parseExtraInt(extra["codex_"+kind+"_window_minutes"]); wm > 0 {
				duration = time.Duration(wm) * time.Minute
			}
			usedRatio := used / 100
			resetAt, usedRatio = recoverExpiredCapacityWindow(resetAt, usedRatio, duration, now)
			used = usedRatio * 100
			out.ResetAt = &resetAt
			elapsed := duration - resetAt.Sub(now)
			if elapsed > 0 && duration > 0 {
				sustainable := 100.0 * (elapsed.Hours() / duration.Hours())
				if sustainable > 0 && used > 0 {
					out.BurnRate = used / sustainable
				}
				remainingAfter := math.Max(0, 100-used)
				if out.BurnRate > 0 && remainingAfter > 0 {
					hoursLeft := (remainingAfter / 100) / ((used / 100) / math.Max(elapsed.Hours(), 0.01))
					exhaust := now.Add(time.Duration(hoursLeft * float64(time.Hour)))
					out.ExhaustsAt = &exhaust
				}
			}
		}
	}
	remaining := math.Max(0, 100-used)
	out.UsedPercent = &used
	out.RemainingPercent = &remaining
	out.projectedGap = out.ExhaustsAt != nil && out.ResetAt != nil && out.ExhaustsAt.Before(*out.ResetAt)
	out.Alert = openAIOAuthAlertLevel(remaining, out.BurnRate, out.BurnRate, out.projectedGap)
	return out
}

func openAIOAuthAlertLevel(remaining, shortBurn, longBurn float64, projectedGap bool) string {
	if remaining <= openAIOAuthCriticalRemaining && shortBurn >= 1.5 && longBurn >= 1.2 {
		return "critical"
	}
	if remaining <= openAIOAuthWarningRemainingPct && shortBurn >= 1 && longBurn >= 1 {
		return "warning"
	}
	if projectedGap && shortBurn >= 1 && longBurn >= 1 {
		return "warning"
	}
	if remaining > 0 && remaining <= 25 && shortBurn >= 1.2 && longBurn >= 1.2 {
		return "warning"
	}
	return ""
}

func openAIOAuthRateLimitBuckets(accounts []Account, now time.Time) OpenAIOAuthRateLimitBuckets {
	var buckets OpenAIOAuthRateLimitBuckets
	for i := range accounts {
		reset := accounts[i].RateLimitResetAt
		if reset == nil || !reset.After(now) {
			continue
		}
		buckets.Total++
		until := reset.Sub(now)
		switch {
		case until <= 10*time.Minute:
			buckets.UpTo10Minutes++
		case until <= 30*time.Minute:
			buckets.From10To30++
		case until <= time.Hour:
			buckets.From30To1Hour++
		case until <= 3*time.Hour:
			buckets.From1To3Hours++
		case until <= 5*time.Hour:
			buckets.From3To5Hours++
		case until <= 24*time.Hour:
			buckets.From5HoursTo1Day++
		case until <= 72*time.Hour:
			buckets.From1To3Days++
		default:
			buckets.Over3Days++
		}
	}
	return buckets
}

func normalizeOpenAIOAuthPlan(plan string) string {
	plan = strings.ToLower(strings.TrimSpace(plan))
	if plan == "" {
		return "unknown"
	}
	return plan
}
