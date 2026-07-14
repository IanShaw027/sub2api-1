package main

import (
	"context"
	"database/sql"
	"flag"
	"fmt"
	"log"
	"strconv"
	"strings"
	"time"

	"entgo.io/ent/dialect"
	entsql "entgo.io/ent/dialect/sql"
	dbent "github.com/Wei-Shaw/sub2api/ent"
	_ "github.com/Wei-Shaw/sub2api/ent/runtime"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/pkg/timezone"
	"github.com/Wei-Shaw/sub2api/internal/repository"
	"github.com/Wei-Shaw/sub2api/internal/service"
	_ "github.com/lib/pq"
)

type failedAffiliateOrder struct {
	ID    int64
	Email string
}

func main() {
	var (
		orderIDsRaw string
		dryRun      bool
	)
	flag.StringVar(&orderIDsRaw, "order-ids", "", "comma-separated order IDs to repair; defaults to all affected orders")
	flag.BoolVar(&dryRun, "dry-run", false, "list affected orders without retrying fulfillment")
	flag.Parse()

	cfg, err := config.ProvideConfig()
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	client, sqlDB, err := openEntClientWithoutMigrations(cfg)
	if err != nil {
		log.Fatalf("init ent client: %v", err)
	}
	defer func() { _ = client.Close() }()

	settingRepo := repository.NewSettingRepository(client)
	affiliateRepo := repository.NewAffiliateRepository(client, sqlDB)
	affiliateSvc := service.NewAffiliateService(affiliateRepo, settingRepo, nil, nil)
	paymentSvc := service.NewPaymentService(client, nil, nil, nil, nil, nil, nil, nil, affiliateSvc)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()

	orderIDs, err := parseOrderIDs(orderIDsRaw)
	if err != nil {
		log.Fatalf("parse order ids: %v", err)
	}

	orders, err := loadAffectedOrders(ctx, client, orderIDs)
	if err != nil {
		log.Fatalf("load affected orders: %v", err)
	}
	if len(orders) == 0 {
		log.Println("no affected affiliate rebate orders found")
		return
	}

	log.Printf("found %d affected orders", len(orders))
	for _, order := range orders {
		log.Printf("order=%d email=%s", order.ID, order.Email)
	}
	if dryRun {
		return
	}

	var repaired, failed int
	for _, order := range orders {
		if err := paymentSvc.RetryFulfillment(ctx, order.ID); err != nil {
			failed++
			log.Printf("repair failed order=%d email=%s err=%v", order.ID, order.Email, err)
			continue
		}
		repaired++
		log.Printf("repair ok order=%d email=%s", order.ID, order.Email)
	}

	log.Printf("repair finished repaired=%d failed=%d", repaired, failed)
}

func openEntClientWithoutMigrations(cfg *config.Config) (*dbent.Client, *sql.DB, error) {
	if err := timezone.Init(cfg.Timezone); err != nil {
		return nil, nil, err
	}
	dsn := cfg.Database.DSNWithTimezone(cfg.Timezone)
	drv, err := entsql.Open(dialect.Postgres, dsn)
	if err != nil {
		return nil, nil, err
	}
	client := dbent.NewClient(dbent.Driver(drv))
	return client, drv.DB(), nil
}

func parseOrderIDs(raw string) ([]int64, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return nil, nil
	}
	parts := strings.Split(raw, ",")
	out := make([]int64, 0, len(parts))
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		id, err := strconv.ParseInt(part, 10, 64)
		if err != nil || id <= 0 {
			return nil, fmt.Errorf("invalid order id %q", part)
		}
		out = append(out, id)
	}
	return out, nil
}

func loadAffectedOrders(ctx context.Context, client *dbent.Client, orderIDs []int64) ([]failedAffiliateOrder, error) {
	query, args := buildAffectedOrdersQuery(orderIDs)
	rows, err := client.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	out := make([]failedAffiliateOrder, 0)
	for rows.Next() {
		var item failedAffiliateOrder
		if err := rows.Scan(&item.ID, &item.Email); err != nil {
			return nil, err
		}
		out = append(out, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return out, nil
}

func buildAffectedOrdersQuery(orderIDs []int64) (string, []any) {
	args := make([]any, 0, len(orderIDs))
	filterSQL := ""
	if len(orderIDs) > 0 {
		placeholders := make([]string, 0, len(orderIDs))
		for _, id := range orderIDs {
			args = append(args, id)
			placeholders = append(placeholders, fmt.Sprintf("$%d", len(args)))
		}
		filterSQL = " AND po.id IN (" + strings.Join(placeholders, ", ") + ")"
	}

	return `
SELECT po.id, COALESCE(u.email, '')
FROM payment_orders po
JOIN users u
  ON u.id = po.user_id
WHERE po.order_type IN ('balance', 'subscription')
  AND po.status = 'COMPLETED'
  AND NOT EXISTS (
    SELECT 1
    FROM payment_audit_logs ok
    WHERE ok.order_id = po.id::text
      AND ok.action IN ('AFFILIATE_REBATE_APPLIED', 'AFFILIATE_REBATE_SKIPPED')
  )` + filterSQL + `
ORDER BY po.id`, args
}
