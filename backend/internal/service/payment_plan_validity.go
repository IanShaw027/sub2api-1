package service

import "strings"

const (
	validityUnitDay   = "day"
	validityUnitWeek  = "week"
	validityUnitMonth = "month"
	validityUnitYear  = "year"
)

func normalizePlanValidityUnit(unit string) string {
	switch strings.ToLower(strings.TrimSpace(unit)) {
	case validityUnitDay, "days":
		return validityUnitDay
	case validityUnitWeek, "weeks":
		return validityUnitWeek
	case validityUnitMonth, "months":
		return validityUnitMonth
	case validityUnitYear, "years":
		return validityUnitYear
	default:
		return ""
	}
}

func computePlanValidityDays(days int, unit string) int {
	switch normalizePlanValidityUnit(unit) {
	case validityUnitWeek:
		return days * 7
	case validityUnitMonth:
		return days * 30
	case validityUnitYear:
		return days * 365
	default:
		return days
	}
}
