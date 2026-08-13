package repository

import (
	"context"
	"strings"
	"time"

	entsql "entgo.io/ent/dialect/sql"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/announcementread"
	"github.com/Wei-Shaw/sub2api/ent/apikey"
	"github.com/Wei-Shaw/sub2api/ent/predicate"
	dbuser "github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/ent/usersubscription"
	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
	"github.com/Wei-Shaw/sub2api/internal/service"
)

type announcementReadRepository struct {
	client *dbent.Client
}

func NewAnnouncementReadRepository(client *dbent.Client) service.AnnouncementReadRepository {
	return &announcementReadRepository{client: client}
}

func (r *announcementReadRepository) MarkRead(ctx context.Context, announcementID, userID int64, readAt time.Time) error {
	client := clientFromContext(ctx, r.client)
	err := client.AnnouncementRead.Create().
		SetAnnouncementID(announcementID).
		SetUserID(userID).
		SetReadAt(readAt).
		OnConflictColumns(announcementread.FieldAnnouncementID, announcementread.FieldUserID).
		DoNothing().
		Exec(ctx)
	if isSQLNoRowsError(err) {
		return nil
	}
	return err
}

func (r *announcementReadRepository) GetReadMapByUser(ctx context.Context, userID int64, announcementIDs []int64) (map[int64]time.Time, error) {
	if len(announcementIDs) == 0 {
		return map[int64]time.Time{}, nil
	}

	rows, err := r.client.AnnouncementRead.Query().
		Where(
			announcementread.UserIDEQ(userID),
			announcementread.AnnouncementIDIn(announcementIDs...),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := make(map[int64]time.Time, len(rows))
	for i := range rows {
		out[rows[i].AnnouncementID] = rows[i].ReadAt
	}
	return out, nil
}

func (r *announcementReadRepository) GetReadMapByUsers(ctx context.Context, announcementID int64, userIDs []int64) (map[int64]time.Time, error) {
	if len(userIDs) == 0 {
		return map[int64]time.Time{}, nil
	}

	rows, err := r.client.AnnouncementRead.Query().
		Where(
			announcementread.AnnouncementIDEQ(announcementID),
			announcementread.UserIDIn(userIDs...),
		).
		All(ctx)
	if err != nil {
		return nil, err
	}

	out := make(map[int64]time.Time, len(rows))
	for i := range rows {
		out[rows[i].UserID] = rows[i].ReadAt
	}
	return out, nil
}

func (r *announcementReadRepository) CountByAnnouncementID(ctx context.Context, announcementID int64) (int64, error) {
	count, err := r.client.AnnouncementRead.Query().
		Where(announcementread.AnnouncementIDEQ(announcementID)).
		Count(ctx)
	if err != nil {
		return 0, err
	}
	return int64(count), nil
}

func (r *announcementReadRepository) ListUserReadStatus(ctx context.Context, announcementID int64, targeting service.AnnouncementTargeting, now time.Time, params pagination.PaginationParams, search, readStatus string) ([]service.AnnouncementUserReadStatus, *pagination.PaginationResult, error) {
	q := r.client.User.Query().Where(announcementAudiencePredicate(targeting, now))
	if search = strings.TrimSpace(search); search != "" {
		q = q.Where(dbuser.Or(
			dbuser.EmailContainsFold(search),
			dbuser.UsernameContainsFold(search),
			dbuser.NotesContainsFold(search),
			dbuser.HasAPIKeysWith(apikey.KeyContainsFold(search), apikey.DeletedAtIsNil()),
		))
	}
	switch service.NormalizeAnnouncementReadStatus(readStatus) {
	case service.AnnouncementReadStatusRead:
		q = q.Where(dbuser.HasAnnouncementReadsWith(announcementread.AnnouncementIDEQ(announcementID)))
	case service.AnnouncementReadStatusUnread:
		q = q.Where(dbuser.Not(dbuser.HasAnnouncementReadsWith(announcementread.AnnouncementIDEQ(announcementID))))
	}

	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, nil, err
	}
	usersQuery := q.Offset(params.Offset()).Limit(params.Limit())
	for _, order := range userListOrder(params) {
		usersQuery = usersQuery.Order(order)
	}
	users, err := usersQuery.All(ctx)
	if err != nil {
		return nil, nil, err
	}
	userIDs := make([]int64, 0, len(users))
	for _, user := range users {
		userIDs = append(userIDs, user.ID)
	}
	readMap, err := r.GetReadMapByUsers(ctx, announcementID, userIDs)
	if err != nil {
		return nil, nil, err
	}
	items := make([]service.AnnouncementUserReadStatus, 0, len(users))
	for _, user := range users {
		var readAt *time.Time
		if value, ok := readMap[user.ID]; ok {
			valueCopy := value
			readAt = &valueCopy
		}
		items = append(items, service.AnnouncementUserReadStatus{
			UserID:   user.ID,
			Email:    user.Email,
			Username: user.Username,
			Balance:  user.Balance,
			Eligible: true,
			ReadAt:   readAt,
		})
	}
	return items, paginationResultFromTotal(int64(total), params), nil
}

func announcementAudiencePredicate(targeting service.AnnouncementTargeting, now time.Time) predicate.User {
	if len(targeting.AnyOf) == 0 {
		return func(*entsql.Selector) {}
	}
	groups := make([]predicate.User, 0, len(targeting.AnyOf))
	for _, group := range targeting.AnyOf {
		if len(group.AllOf) == 0 {
			continue
		}
		conditions := make([]predicate.User, 0, len(group.AllOf))
		valid := true
		for _, condition := range group.AllOf {
			switch condition.Type {
			case domain.AnnouncementConditionTypeSubscription:
				if condition.Operator != domain.AnnouncementOperatorIn || len(condition.GroupIDs) == 0 {
					valid = false
					break
				}
				conditions = append(conditions, dbuser.HasSubscriptionsWith(
					usersubscription.StatusEQ(service.SubscriptionStatusActive),
					usersubscription.ExpiresAtGT(now),
					usersubscription.GroupIDIn(condition.GroupIDs...),
					usersubscription.DeletedAtIsNil(),
				))
			case domain.AnnouncementConditionTypeBalance:
				switch condition.Operator {
				case domain.AnnouncementOperatorGT:
					conditions = append(conditions, dbuser.BalanceGT(condition.Value))
				case domain.AnnouncementOperatorGTE:
					conditions = append(conditions, dbuser.BalanceGTE(condition.Value))
				case domain.AnnouncementOperatorLT:
					conditions = append(conditions, dbuser.BalanceLT(condition.Value))
				case domain.AnnouncementOperatorLTE:
					conditions = append(conditions, dbuser.BalanceLTE(condition.Value))
				case domain.AnnouncementOperatorEQ:
					conditions = append(conditions, dbuser.BalanceEQ(condition.Value))
				default:
					valid = false
				}
			default:
				valid = false
			}
			if !valid {
				break
			}
		}
		if valid && len(conditions) > 0 {
			groups = append(groups, dbuser.And(conditions...))
		}
	}
	if len(groups) == 0 {
		return func(selector *entsql.Selector) { selector.Where(entsql.False()) }
	}
	return dbuser.Or(groups...)
}
