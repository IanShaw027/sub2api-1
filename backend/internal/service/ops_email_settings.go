package service

import (
	"context"
	"strings"
)

func opsEmailSeverityRank(severity string) int {
	switch strings.ToLower(strings.TrimSpace(severity)) {
	case "critical":
		return 3
	case "warning":
		return 2
	case "info":
		return 1
	default:
		return 0
	}
}

func shouldSendOpsEmailBySeverity(minSeverity string, actualSeverity string) bool {
	min := strings.TrimSpace(minSeverity)
	if min == "" {
		return true
	}
	return opsEmailSeverityRank(actualSeverity) >= opsEmailSeverityRank(min)
}

func resolveOpsAlertEmailRecipients(ctx context.Context, userService *UserService, cfg *OpsEmailNotificationConfig) ([]string, error) {
	if cfg != nil && len(cfg.Alert.Recipients) > 0 {
		return normalizeEmails(cfg.Alert.Recipients), nil
	}
	if userService == nil {
		return []string{}, nil
	}
	admin, err := userService.GetFirstAdmin(ctx)
	if err != nil || admin == nil || strings.TrimSpace(admin.Email) == "" {
		return []string{}, err
	}
	return []string{strings.TrimSpace(admin.Email)}, nil
}

func resolveOpsReportEmailRecipients(ctx context.Context, userService *UserService, cfg *OpsEmailNotificationConfig) ([]string, error) {
	if cfg != nil && len(cfg.Report.Recipients) > 0 {
		return normalizeEmails(cfg.Report.Recipients), nil
	}
	if userService == nil {
		return []string{}, nil
	}
	admin, err := userService.GetFirstAdmin(ctx)
	if err != nil || admin == nil || strings.TrimSpace(admin.Email) == "" {
		return []string{}, err
	}
	return []string{strings.TrimSpace(admin.Email)}, nil
}

func normalizeEmails(emails []string) []string {
	if len(emails) == 0 {
		return []string{}
	}
	seen := make(map[string]struct{}, len(emails))
	out := make([]string, 0, len(emails))
	for _, e := range emails {
		trimmed := strings.ToLower(strings.TrimSpace(e))
		if trimmed == "" {
			continue
		}
		if _, ok := seen[trimmed]; ok {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	return out
}
