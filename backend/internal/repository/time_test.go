//go:build integration

package repository

import "time"

func ptrTime(t time.Time) *time.Time {
	return &t
}
