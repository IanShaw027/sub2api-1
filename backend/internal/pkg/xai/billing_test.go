package xai

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBuildBillingURL(t *testing.T) {
	u, err := BuildBillingURL(DefaultCLIBaseURL, BillingPathCredits)
	require.NoError(t, err)
	require.Equal(t, "https://cli-chat-proxy.grok.com/v1/billing?format=credits", u)

	u, err = BuildBillingURL(DefaultCLIBaseURL+"/", BillingPathMonthly)
	require.NoError(t, err)
	require.Equal(t, "https://cli-chat-proxy.grok.com/v1/billing", u)
}

func TestParseCreditsAndMonthly(t *testing.T) {
	creditsBody := []byte(`{
	  "config": {
	    "currentPeriod": {
	      "type": "USAGE_PERIOD_TYPE_WEEKLY",
	      "start": "2026-07-13T08:44:35.786327+00:00",
	      "end": "2026-07-20T08:44:35.786327+00:00"
	    },
	    "creditUsagePercent": 55.0,
	    "onDemandCap": {"val": 0},
	    "onDemandUsed": {"val": 0},
	    "productUsage": [{"product": "Api", "usagePercent": 55.0}],
	    "isUnifiedBillingUser": true,
	    "prepaidBalance": {"val": 0},
	    "topUpMethod": "TOP_UP_METHOD_SAVED_PAYMENT_METHOD"
	  }
	}`)
	credits, err := ParseCreditsBilling(creditsBody)
	require.NoError(t, err)
	require.NotNil(t, credits.Config)
	require.InDelta(t, 55.0, WeeklyUtilization(credits.Config), 0.001)
	start, end := WeeklyPeriodBounds(credits.Config)
	require.NotNil(t, start)
	require.NotNil(t, end)
	require.True(t, end.After(*start))

	monthlyBody := []byte(`{
	  "config": {
	    "monthlyLimit": {"val": 15000},
	    "used": {"val": 2192},
	    "onDemandCap": {"val": 0},
	    "billingPeriodStart": "2026-07-01T00:00:00+00:00",
	    "billingPeriodEnd": "2026-08-01T00:00:00+00:00"
	  }
	}`)
	monthly, err := ParseMonthlyBilling(monthlyBody)
	require.NoError(t, err)
	require.InDelta(t, 14.6133, MonthlyUtilization(monthly.Config), 0.01)
	require.Equal(t, 2192.0, Money(monthly.Config.Used))
	require.Equal(t, 15000.0, Money(monthly.Config.MonthlyLimit))
	periodEnd := MonthlyPeriodEnd(monthly.Config)
	require.NotNil(t, periodEnd)
	require.Equal(t, 2026, periodEnd.Year())
	require.Equal(t, time.August, periodEnd.Month())
}
