package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/group"
	"github.com/Wei-Shaw/sub2api/ent/predicate"
	"github.com/Wei-Shaw/sub2api/ent/supportticket"
	"github.com/Wei-Shaw/sub2api/ent/supportticketmessage"
	"github.com/Wei-Shaw/sub2api/ent/supportticketreplytemplate"
	"github.com/Wei-Shaw/sub2api/ent/user"
	"github.com/Wei-Shaw/sub2api/internal/config"
	"github.com/Wei-Shaw/sub2api/internal/domain"
)

const (
	supportTicketTitleMaxRunes       = 80
	supportTicketTitleMaxBytes       = 240
	supportTicketPayloadMaxBytes     = 16 * 1024
	supportTicketPayloadMaxFields    = 32
	supportTicketPayloadMaxFieldName = 64
	supportTicketPayloadMaxDepth     = 4
	supportTicketReplyMaxBytes       = 8 * 1024
	supportTicketReplyDedupWindow    = 10 * time.Second
	supportTicketMaxAttachments      = 8
	supportTicketStaffDisplayName    = "Support"
	ticketReplyNotifyTimeout         = 45 * time.Second
)

type TicketService struct {
	entClient                *dbent.Client
	frontendURL              string
	userGroupRates           UserGroupRateRepository
	notificationEmailService *NotificationEmailService
}

func NewTicketService(entClient *dbent.Client, cfg *config.Config) *TicketService {
	frontendURL := ""
	if cfg != nil {
		frontendURL = strings.TrimRight(strings.TrimSpace(cfg.Server.FrontendURL), "/")
	}
	return &TicketService{entClient: entClient, frontendURL: frontendURL}
}

func ProvideTicketService(entClient *dbent.Client, cfg *config.Config, notificationEmailService *NotificationEmailService, userGroupRates UserGroupRateRepository) *TicketService {
	svc := NewTicketService(entClient, cfg)
	svc.notificationEmailService = notificationEmailService
	svc.userGroupRates = userGroupRates
	return svc
}

func (s *TicketService) Create(ctx context.Context, input CreateSupportTicketInput) (*SupportTicket, error) {
	category := NormalizeSupportTicketCategory(input.Category)
	if category == "" {
		return nil, ErrTicketInvalidCategory
	}
	title, err := normalizeSupportTicketTitle(input.Title)
	if err != nil {
		return nil, err
	}
	payload, err := s.prepareTicketPayload(ctx, input.UserID, category, input.FormPayload)
	if err != nil {
		return nil, err
	}
	if _, err := s.entClient.User.Get(ctx, input.UserID); err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrTicketForbidden
		}
		return nil, fmt.Errorf("load ticket user: %w", err)
	}
	now := time.Now()
	ticketNo := generateTicketNo(now)
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin ticket create: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	row, err := tx.SupportTicket.Create().
		SetTicketNo(ticketNo).
		SetUserID(input.UserID).
		SetCategory(category).
		SetTitle(title).
		SetStatus(SupportTicketStatusSubmitted).
		SetCurrentFormPayload(payload).
		SetCurrentRevisionNo(1).
		SetLatestMessageAt(now).
		SetLastReplyRole(SupportTicketSenderRoleSystem).
		SetUnreadByUser(false).
		SetUnreadByAdmin(true).
		SetSubmittedAt(now).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create ticket: %w", err)
	}
	if _, err := tx.SupportTicketRevision.Create().
		SetTicketID(row.ID).
		SetRevisionNo(1).
		SetTitle(title).
		SetFormPayload(payload).
		SetSubmittedBy(input.UserID).
		SetSubmittedAt(now).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("create ticket revision: %w", err)
	}
	if err := insertTicketSystemMessage(ctx, tx.Client(), row.ID, "用户提交了工单。", now); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit ticket create: %w", err)
	}
	committed = true
	return s.GetForUser(ctx, input.UserID, row.ID)
}

func (s *TicketService) GetForUser(ctx context.Context, userID, ticketID int64) (*SupportTicket, error) {
	ticket, err := s.loadTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.UserID != userID {
		return nil, ErrTicketForbidden
	}
	if ticket.UnreadByUser {
		cleared, err := s.entClient.SupportTicket.Update().
			Where(
				supportticket.IDEQ(ticketID),
				supportticket.UnreadByUserEQ(true),
				supportticket.LatestMessageAtEQ(ticket.LatestMessageAt),
			).
			SetUnreadByUser(false).
			Save(ctx)
		if err == nil && cleared > 0 {
			ticket.UnreadByUser = false
		}
	}
	return ticket, nil
}

func (s *TicketService) GetForAdmin(ctx context.Context, ticketID int64) (*SupportTicket, error) {
	ticket, err := s.loadTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.UnreadByAdmin {
		cleared, err := s.entClient.SupportTicket.Update().
			Where(
				supportticket.IDEQ(ticketID),
				supportticket.UnreadByAdminEQ(true),
				supportticket.LatestMessageAtEQ(ticket.LatestMessageAt),
			).
			SetUnreadByAdmin(false).
			Save(ctx)
		if err == nil && cleared > 0 {
			ticket.UnreadByAdmin = false
		}
	}
	return ticket, nil
}

func (s *TicketService) ListForUser(ctx context.Context, userID int64, filters SupportTicketListFilters) ([]SupportTicket, int, error) {
	filters.UserID = userID
	return s.list(ctx, filters, false)
}

func (s *TicketService) ListForAdmin(ctx context.Context, filters SupportTicketListFilters) ([]SupportTicket, int, error) {
	return s.list(ctx, filters, true)
}

func (s *TicketService) CountUnreadForUser(ctx context.Context, userID int64) (int, error) {
	return s.entClient.SupportTicket.Query().
		Where(
			supportticket.UserIDEQ(userID),
			supportticket.UnreadByUserEQ(true),
			supportticket.StatusNotIn(ticketReminderExcludedStatuses()...),
		).
		Count(ctx)
}

func (s *TicketService) CountUnreadForAdmin(ctx context.Context) (int, error) {
	return s.entClient.SupportTicket.Query().
		Where(
			supportticket.UnreadByAdminEQ(true),
			supportticket.StatusNotIn(ticketReminderExcludedStatuses()...),
		).
		Count(ctx)
}

func ticketReminderExcludedStatuses() []string {
	return []string{SupportTicketStatusResolved, SupportTicketStatusClosed, SupportTicketStatusWithdrawn}
}

func (s *TicketService) ListMessagesForUser(ctx context.Context, userID, ticketID int64) ([]SupportTicketMessage, error) {
	ticket, err := s.loadTicket(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.UserID != userID {
		return nil, ErrTicketForbidden
	}
	messages, err := s.listMessages(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	for i := range messages {
		if messages[i].SenderRole != SupportTicketSenderRoleUser {
			messages[i].SenderUserID = nil
		}
	}
	return messages, nil
}

func (s *TicketService) ListMessagesForAdmin(ctx context.Context, ticketID int64) ([]SupportTicketMessage, error) {
	if _, err := s.loadTicket(ctx, ticketID); err != nil {
		return nil, err
	}
	return s.listMessages(ctx, ticketID)
}

func (s *TicketService) Withdraw(ctx context.Context, userID, ticketID int64) error {
	ticket, err := s.loadTicket(ctx, ticketID)
	if err != nil {
		return err
	}
	if ticket.UserID != userID {
		return ErrTicketForbidden
	}
	if ticket.Status != SupportTicketStatusSubmitted && ticket.Status != SupportTicketStatusProcessing && ticket.Status != SupportTicketStatusWaitingAdmin {
		return ErrTicketCannotWithdraw
	}
	now := time.Now()
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	updated, err := tx.SupportTicket.Update().
		Where(supportticket.IDEQ(ticketID), supportticket.StatusIn(SupportTicketStatusSubmitted, SupportTicketStatusProcessing, SupportTicketStatusWaitingAdmin)).
		SetStatus(SupportTicketStatusWithdrawn).
		SetWithdrawnAt(now).
		SetLatestMessageAt(now).
		SetLastReplyRole(SupportTicketSenderRoleSystem).
		SetUnreadByUser(false).
		SetUnreadByAdmin(false).
		Save(ctx)
	if err != nil {
		return err
	}
	if updated == 0 {
		return ErrTicketCannotWithdraw
	}
	if err := insertTicketSystemMessage(ctx, tx.Client(), ticketID, "用户撤回了工单，进入可修改状态。", now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *TicketService) UpdateEditable(ctx context.Context, ticketID int64, input UpdateSupportTicketInput) error {
	if input.ExpectedRevisionNo <= 0 {
		return ErrTicketRevisionRequired
	}
	ticket, err := s.loadTicket(ctx, ticketID)
	if err != nil {
		return err
	}
	if ticket.UserID != input.UserID {
		return ErrTicketForbidden
	}
	if ticket.Status != SupportTicketStatusWithdrawn {
		return ErrTicketNotEditable
	}
	title, err := normalizeSupportTicketTitle(input.Title)
	if err != nil {
		return err
	}
	payload, err := s.prepareTicketPayload(ctx, input.UserID, ticket.Category, input.FormPayload)
	if err != nil {
		return err
	}
	updated, err := s.entClient.SupportTicket.Update().
		Where(
			supportticket.IDEQ(ticketID),
			supportticket.StatusEQ(SupportTicketStatusWithdrawn),
			supportticket.CurrentRevisionNoEQ(input.ExpectedRevisionNo),
		).
		SetTitle(title).
		SetCurrentFormPayload(payload).
		SetCurrentRevisionNo(input.ExpectedRevisionNo + 1).
		Save(ctx)
	if err != nil {
		return err
	}
	if updated == 0 {
		return ErrTicketRevisionConflict
	}
	return nil
}

func (s *TicketService) Resubmit(ctx context.Context, ticketID int64, input UpdateSupportTicketInput) error {
	if input.ExpectedRevisionNo <= 0 {
		return ErrTicketRevisionRequired
	}
	ticket, err := s.loadTicket(ctx, ticketID)
	if err != nil {
		return err
	}
	if ticket.UserID != input.UserID {
		return ErrTicketForbidden
	}
	if ticket.Status != SupportTicketStatusWithdrawn {
		return ErrTicketNotEditable
	}
	title, err := normalizeSupportTicketTitle(input.Title)
	if err != nil {
		return err
	}
	payload, err := s.prepareTicketPayload(ctx, input.UserID, ticket.Category, input.FormPayload)
	if err != nil {
		return err
	}
	now := time.Now()
	nextRevision := input.ExpectedRevisionNo + 1
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	updated, err := tx.SupportTicket.Update().
		Where(
			supportticket.IDEQ(ticketID),
			supportticket.StatusEQ(SupportTicketStatusWithdrawn),
			supportticket.CurrentRevisionNoEQ(input.ExpectedRevisionNo),
		).
		SetTitle(title).
		SetStatus(SupportTicketStatusSubmitted).
		SetCurrentFormPayload(payload).
		SetCurrentRevisionNo(nextRevision).
		SetLatestMessageAt(now).
		SetLastReplyRole(SupportTicketSenderRoleSystem).
		SetUnreadByUser(false).
		SetUnreadByAdmin(true).
		SetSubmittedAt(now).
		ClearWithdrawnAt().
		ClearClosedAt().
		Save(ctx)
	if err != nil {
		return err
	}
	if updated == 0 {
		return ErrTicketRevisionConflict
	}
	if _, err := tx.SupportTicketRevision.Create().
		SetTicketID(ticketID).
		SetRevisionNo(nextRevision).
		SetTitle(title).
		SetFormPayload(payload).
		SetSubmittedBy(input.UserID).
		SetSubmittedAt(now).
		Save(ctx); err != nil {
		return err
	}
	if err := insertTicketSystemMessage(ctx, tx.Client(), ticketID, "用户更新了工单内容并重新提交。", now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *TicketService) CloseForUser(ctx context.Context, userID, ticketID int64) error {
	ticket, err := s.loadTicket(ctx, ticketID)
	if err != nil {
		return err
	}
	if ticket.UserID != userID {
		return ErrTicketForbidden
	}
	if ticket.Status == SupportTicketStatusClosed || ticket.Status == SupportTicketStatusWithdrawn {
		return ErrTicketCannotClose
	}
	now := time.Now()
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	updated, err := tx.SupportTicket.Update().
		Where(supportticket.IDEQ(ticketID), supportticket.StatusNotIn(SupportTicketStatusClosed, SupportTicketStatusWithdrawn)).
		SetStatus(SupportTicketStatusClosed).
		SetClosedAt(now).
		SetLatestMessageAt(now).
		SetLastReplyRole(SupportTicketSenderRoleSystem).
		SetUnreadByUser(false).
		SetUnreadByAdmin(true).
		ClearWithdrawnAt().
		Save(ctx)
	if err != nil {
		return err
	}
	if updated == 0 {
		return ErrTicketCannotClose
	}
	if err := insertTicketSystemMessage(ctx, tx.Client(), ticketID, "用户关闭了工单。", now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *TicketService) ReplyForUser(ctx context.Context, ticketID int64, input CreateSupportTicketMessageInput) error {
	ticket, err := s.loadTicket(ctx, ticketID)
	if err != nil {
		return err
	}
	if ticket.UserID != input.UserID {
		return ErrTicketForbidden
	}
	return s.addReply(ctx, ticket, input, SupportTicketSenderRoleUser, false, true, SupportTicketStatusWaitingAdmin)
}

func (s *TicketService) ReplyForAdmin(ctx context.Context, ticketID int64, input CreateSupportTicketMessageInput) error {
	ticket, err := s.loadTicket(ctx, ticketID)
	if err != nil {
		return err
	}
	return s.addReply(ctx, ticket, input, SupportTicketSenderRoleAdmin, true, false, SupportTicketStatusWaitingUser)
}

func (s *TicketService) UpdateStatusByAdmin(ctx context.Context, ticketID int64, status string) error {
	next := NormalizeSupportTicketStatus(status)
	if next == "" || next == SupportTicketStatusSubmitted || next == SupportTicketStatusWithdrawn {
		return ErrTicketInvalidStatus
	}
	content := adminTicketStatusMessage(next)
	if content == "" {
		return ErrTicketInvalidStatus
	}
	now := time.Now()
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	locked, err := queryTicketForUpdate(ctx, tx.Client(), ticketID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return ErrTicketNotFound
		}
		return err
	}
	if locked.Status == SupportTicketStatusClosed && next != SupportTicketStatusClosed {
		return ErrTicketStatusLocked
	}
	if locked.Status == SupportTicketStatusResolved && next != SupportTicketStatusResolved && next != SupportTicketStatusClosed {
		return ErrTicketStatusLocked
	}
	if !isAdminManualStatusTransitionAllowed(locked.Status, locked.LastReplyRole, next) {
		return ErrTicketStatusInvalidTransition
	}
	upd := tx.SupportTicket.Update().
		Where(
			supportticket.IDEQ(ticketID),
			supportticket.StatusEQ(locked.Status),
			supportticket.LastReplyRoleEQ(locked.LastReplyRole),
		).
		SetStatus(next).
		SetLatestMessageAt(now).
		SetUnreadByUser(true).
		SetUnreadByAdmin(false)
	if next == SupportTicketStatusResolved || next == SupportTicketStatusClosed {
		upd.SetClosedAt(now)
	} else {
		upd.ClearClosedAt()
	}
	updated, err := upd.Save(ctx)
	if err != nil {
		return err
	}
	if updated == 0 {
		return ErrTicketStatusInvalidTransition
	}
	if err := insertTicketSystemMessage(ctx, tx.Client(), ticketID, content, now); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *TicketService) ListReplyTemplates(ctx context.Context) ([]TicketReplyTemplate, error) {
	rows, err := s.entClient.SupportTicketReplyTemplate.Query().
		Order(supportticketreplytemplate.BySortOrder(), supportticketreplytemplate.ByID()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]TicketReplyTemplate, 0, len(rows))
	for _, row := range rows {
		out = append(out, ticketTemplateToView(row))
	}
	return out, nil
}

func (s *TicketService) CreateReplyTemplate(ctx context.Context, title, content string, sortOrder int) (*TicketReplyTemplate, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if title == "" || content == "" || utf8.RuneCountInString(title) > 120 {
		return nil, ErrTicketTemplateInvalid
	}
	row, err := s.entClient.SupportTicketReplyTemplate.Create().
		SetTitle(title).
		SetContent(content).
		SetSortOrder(sortOrder).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	view := ticketTemplateToView(row)
	return &view, nil
}

func (s *TicketService) UpdateReplyTemplate(ctx context.Context, id int64, title, content string, sortOrder int) (*TicketReplyTemplate, error) {
	title = strings.TrimSpace(title)
	content = strings.TrimSpace(content)
	if title == "" || content == "" || utf8.RuneCountInString(title) > 120 {
		return nil, ErrTicketTemplateInvalid
	}
	row, err := s.entClient.SupportTicketReplyTemplate.UpdateOneID(id).
		SetTitle(title).
		SetContent(content).
		SetSortOrder(sortOrder).
		Save(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	view := ticketTemplateToView(row)
	return &view, nil
}

func (s *TicketService) DeleteReplyTemplate(ctx context.Context, id int64) error {
	err := s.entClient.SupportTicketReplyTemplate.DeleteOneID(id).Exec(ctx)
	if dbent.IsNotFound(err) {
		return ErrTicketNotFound
	}
	return err
}

func (s *TicketService) ListRateApplyGroups(ctx context.Context, userID int64) ([]TicketRateGroupOption, error) {
	rows, err := s.entClient.Group.Query().
		Where(
			group.StatusEQ(domain.StatusActive),
			group.SubscriptionTypeEQ(domain.SubscriptionTypeStandard),
			group.DeletedAtIsNil(),
		).
		Order(group.ByName()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	userRates := map[int64]float64{}
	if s.userGroupRates != nil && userID > 0 {
		if rates, rateErr := s.userGroupRates.GetByUserID(ctx, userID); rateErr == nil {
			userRates = rates
		}
	}
	out := make([]TicketRateGroupOption, 0, len(rows))
	for _, row := range rows {
		opt := TicketRateGroupOption{
			GroupID:            row.ID,
			Name:               row.Name,
			BaseRateMultiplier: row.RateMultiplier,
			EffectiveRate:      row.RateMultiplier,
		}
		if special, ok := userRates[row.ID]; ok {
			opt.UserRateMultiplier = &special
			opt.EffectiveRate = special
		}
		out = append(out, opt)
	}
	return out, nil
}

func (s *TicketService) addReply(ctx context.Context, ticket *SupportTicket, input CreateSupportTicketMessageInput, role string, unreadByUser, unreadByAdmin bool, nextStatus string) error {
	if isTicketReplyLocked(ticket.Status) {
		return ErrTicketReplyLocked
	}
	content := strings.TrimSpace(input.Content)
	attachments, err := s.resolveTicketAttachments(ctx, ticket, input)
	if err != nil {
		return err
	}
	if err := validateSupportTicketReplyContent(content, len(attachments) > 0); err != nil {
		return err
	}
	u, err := s.entClient.User.Get(ctx, input.UserID)
	if err != nil {
		return fmt.Errorf("load reply user: %w", err)
	}
	now := time.Now()
	senderID := input.UserID
	message := &SupportTicketMessage{
		SenderRole:           role,
		SenderUserID:         &senderID,
		SenderNameSnapshot:   ticketSenderDisplayName(role, u.Username, u.Email),
		SenderAvatarSnapshot: s.loadTicketUserAvatarURLBestEffort(ctx, input.UserID),
		MessageType:          SupportTicketMessageTypeMessage,
		Content:              content,
		Attachments:          attachments,
		CreatedAt:            now,
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return err
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	locked, err := queryTicketForUpdate(ctx, tx.Client(), ticket.ID)
	if err != nil {
		return err
	}
	if isTicketReplyLocked(locked.Status) {
		return ErrTicketReplyLocked
	}
	dup, err := isRecentDuplicateTicketReply(ctx, tx.Client(), ticket.ID, message)
	if err != nil {
		return err
	}
	if dup {
		if err := tx.Commit(); err != nil {
			return err
		}
		committed = true
		return nil
	}
	raw, _ := json.Marshal(attachments)
	if raw == nil {
		raw = []byte("[]")
	}
	created, err := tx.SupportTicketMessage.Create().
		SetTicketID(ticket.ID).
		SetSenderRole(role).
		SetSenderUserID(input.UserID).
		SetSenderNameSnapshot(message.SenderNameSnapshot).
		SetSenderAvatarSnapshot(message.SenderAvatarSnapshot).
		SetMessageType(SupportTicketMessageTypeMessage).
		SetContent(content).
		SetAttachments(raw).
		SetCreatedAt(now).
		Save(ctx)
	if err != nil {
		return err
	}
	message.ID = created.ID
	updated, err := tx.SupportTicket.Update().
		Where(
			supportticket.IDEQ(ticket.ID),
			supportticket.StatusNotIn(SupportTicketStatusResolved, SupportTicketStatusClosed, SupportTicketStatusWithdrawn),
		).
		SetLatestMessageAt(now).
		SetLastReplyRole(role).
		SetUnreadByUser(unreadByUser).
		SetUnreadByAdmin(unreadByAdmin).
		SetStatus(nextStatus).
		Save(ctx)
	if err != nil {
		return err
	}
	if updated == 0 {
		return ErrTicketReplyLocked
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	s.dispatchTicketReplyNotification(ctx, ticket, message, role)
	return nil
}

func (s *TicketService) resolveTicketAttachments(ctx context.Context, ticket *SupportTicket, input CreateSupportTicketMessageInput) ([]TicketMessageAttachment, error) {
	ids := uniquePositiveIDs(input.MediaIDs)
	if len(ids) == 0 && len(input.Attachments) > 0 {
		for _, item := range input.Attachments {
			ids = append(ids, item.MediaID)
		}
		ids = uniquePositiveIDs(ids)
	}
	if len(ids) == 0 {
		return nil, nil
	}
	if len(ids) > supportTicketMaxAttachments {
		return nil, ErrTicketAttachmentLimit
	}
	out := make([]TicketMessageAttachment, 0, len(ids))
	ticketID := strconv.FormatInt(ticket.ID, 10)
	for _, id := range ids {
		asset, err := s.entClient.MediaAsset.Get(ctx, id)
		if err != nil {
			return nil, ErrTicketAttachmentInvalid
		}
		if asset.Status != MediaStatusReady || asset.BizType != MediaBizTicket || asset.Visibility != MediaVisibilityPrivate {
			return nil, ErrTicketAttachmentInvalid
		}
		if asset.OwnerUserID != ticket.UserID && asset.OwnerUserID != input.UserID {
			return nil, ErrTicketAttachmentInvalid
		}
		bizID := strings.TrimSpace(asset.BizID)
		if bizID != "" && bizID != ticketID {
			return nil, ErrTicketAttachmentInvalid
		}
		if bizID == "" {
			if _, err := s.entClient.MediaAsset.UpdateOneID(asset.ID).SetBizID(ticketID).Save(ctx); err != nil {
				return nil, err
			}
		}
		out = append(out, TicketMessageAttachment{
			MediaID:     asset.ID,
			FileName:    asset.Filename,
			ContentType: asset.Mime,
			SizeBytes:   asset.Size,
		})
	}
	return out, nil
}

func (s *TicketService) EnsureTicketAttachmentAccess(ctx context.Context, ticketID, mediaID, actorUserID int64, isAdmin bool) error {
	ticket, err := s.loadTicket(ctx, ticketID)
	if err != nil {
		return err
	}
	if !isAdmin && ticket.UserID != actorUserID {
		return ErrTicketForbidden
	}
	messages, err := s.listMessages(ctx, ticketID)
	if err != nil {
		return err
	}
	for _, msg := range messages {
		for _, att := range msg.Attachments {
			if att.MediaID == mediaID {
				return nil
			}
		}
	}
	return ErrTicketAttachmentInvalid
}

func (s *TicketService) dispatchTicketReplyNotification(ctx context.Context, ticket *SupportTicket, message *SupportTicketMessage, role string) {
	if s.notificationEmailService == nil || ticket == nil || message == nil {
		return
	}
	ticketCopy := *ticket
	messageCopy := *message
	if message.Attachments != nil {
		messageCopy.Attachments = append([]TicketMessageAttachment(nil), message.Attachments...)
	}
	go func() {
		notifyCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), ticketReplyNotifyTimeout)
		defer cancel()
		s.notifyTicketReply(notifyCtx, &ticketCopy, &messageCopy, role)
	}()
}

func (s *TicketService) notifyTicketReply(ctx context.Context, ticket *SupportTicket, message *SupportTicketMessage, role string) {
	if s.notificationEmailService == nil || ticket == nil || message == nil {
		return
	}
	preview := strings.TrimSpace(message.Content)
	if utf8.RuneCountInString(preview) > 500 {
		preview = string([]rune(preview)[:500]) + "..."
	}
	vars := map[string]string{
		"ticket_no":        ticket.TicketNo,
		"ticket_title":     ticket.Title,
		"sender_name":      message.SenderNameSnapshot,
		"reply_preview":    preview,
		"attachment_count": strconv.Itoa(len(message.Attachments)),
	}
	if role == SupportTicketSenderRoleAdmin {
		if ticket.UserEmail == "" {
			if u, err := s.entClient.User.Get(ctx, ticket.UserID); err == nil {
				ticket.UserEmail = u.Email
				ticket.UserName = firstNonEmpty(u.Username, u.Email)
			}
		}
		if strings.TrimSpace(ticket.UserEmail) == "" {
			return
		}
		vars["ticket_url"] = s.ticketURL("/tickets/" + strconv.FormatInt(ticket.ID, 10))
		if err := s.notificationEmailService.Send(ctx, NotificationEmailSendInput{
			Event:          NotificationEmailEventTicketReply,
			RecipientEmail: ticket.UserEmail,
			RecipientName:  firstNonEmpty(ticket.UserName, ticket.UserEmail),
			UserID:         ticket.UserID,
			SourceType:     "ticket_reply",
			SourceID:       strconv.FormatInt(message.ID, 10),
			Variables:      vars,
		}); err != nil {
			slog.Warn("ticket reply email failed", "ticket_id", ticket.ID, "err", err.Error())
		}
		return
	}
	admins, err := s.entClient.User.Query().
		Where(user.RoleEQ(domain.RoleAdmin), user.StatusEQ(domain.StatusActive)).
		Limit(200).
		All(ctx)
	if err != nil {
		slog.Warn("list ticket admin recipients failed", "ticket_id", ticket.ID, "err", err.Error())
		return
	}
	vars["ticket_url"] = s.ticketURL("/admin/tickets/" + strconv.FormatInt(ticket.ID, 10))
	seen := map[string]struct{}{}
	for _, adminUser := range admins {
		email := strings.TrimSpace(adminUser.Email)
		key := strings.ToLower(email)
		if email == "" {
			continue
		}
		if _, ok := seen[key]; ok {
			continue
		}
		seen[key] = struct{}{}
		if err := s.notificationEmailService.Send(ctx, NotificationEmailSendInput{
			Event:          NotificationEmailEventTicketReply,
			RecipientEmail: email,
			RecipientName:  firstNonEmpty(adminUser.Username, email),
			UserID:         adminUser.ID,
			SourceType:     "ticket_reply",
			SourceID:       strconv.FormatInt(message.ID, 10),
			Variables:      vars,
		}); err != nil {
			slog.Warn("ticket reply admin email failed", "ticket_id", ticket.ID, "err", err.Error())
		}
	}
}

func (s *TicketService) ticketURL(path string) string {
	base := strings.TrimRight(strings.TrimSpace(s.frontendURL), "/")
	if base == "" && s.notificationEmailService != nil {
		base = strings.TrimRight(strings.TrimSpace(s.notificationEmailService.PublicBaseURL(context.Background())), "/")
	}
	if base == "" || !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		return ""
	}
	return base + path
}

func (s *TicketService) loadTicket(ctx context.Context, ticketID int64) (*SupportTicket, error) {
	row, err := s.entClient.SupportTicket.Get(ctx, ticketID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrTicketNotFound
		}
		return nil, err
	}
	view := ticketToView(row, "", "")
	if u, err := s.entClient.User.Get(ctx, row.UserID); err == nil && u != nil {
		view.UserName = firstNonEmpty(u.Username, u.Email)
		view.UserEmail = u.Email
	}
	return &view, nil
}

func (s *TicketService) list(ctx context.Context, filters SupportTicketListFilters, admin bool) ([]SupportTicket, int, error) {
	normalized, err := normalizeSupportTicketListFilters(filters)
	if err != nil {
		return nil, 0, err
	}
	page, pageSize := normalizeInvoicePage(normalized.Page, normalized.PageSize)
	q := s.entClient.SupportTicket.Query()
	if normalized.UserID > 0 {
		q = q.Where(supportticket.UserIDEQ(normalized.UserID))
	}
	if normalized.Status != "" {
		q = q.Where(supportticket.StatusEQ(normalized.Status))
	}
	if normalized.Category != "" {
		q = q.Where(supportticket.CategoryEQ(normalized.Category))
	}
	if admin && normalized.UnreadOnly {
		q = q.Where(supportticket.UnreadByAdminEQ(true))
	}
	if !admin && normalized.UnreadOnly {
		q = q.Where(supportticket.UnreadByUserEQ(true))
	}
	if normalized.StartAt != nil {
		q = q.Where(supportticket.CreatedAtGTE(*normalized.StartAt))
	}
	if normalized.EndAt != nil {
		q = q.Where(supportticket.CreatedAtLT(*normalized.EndAt))
	}
	if kw := strings.TrimSpace(normalized.Search); kw != "" {
		userIDs, _ := s.entClient.User.Query().
			Where(user.Or(user.EmailContainsFold(kw), user.UsernameContainsFold(kw))).
			IDs(ctx)
		preds := []predicate.SupportTicket{
			supportticket.TitleContainsFold(kw),
			supportticket.TicketNoContainsFold(kw),
		}
		if len(userIDs) > 0 {
			preds = append(preds, supportticket.UserIDIn(userIDs...))
		}
		q = q.Where(supportticket.Or(preds...))
	}
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := q.Order(dbent.Desc(supportticket.FieldLatestMessageAt), dbent.Desc(supportticket.FieldID)).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	userIDs := make([]int64, 0, len(rows))
	seen := map[int64]struct{}{}
	for _, row := range rows {
		if _, ok := seen[row.UserID]; ok {
			continue
		}
		seen[row.UserID] = struct{}{}
		userIDs = append(userIDs, row.UserID)
	}
	users := map[int64]*dbent.User{}
	if len(userIDs) > 0 {
		found, _ := s.entClient.User.Query().Where(user.IDIn(userIDs...)).All(ctx)
		for _, u := range found {
			users[u.ID] = u
		}
	}
	out := make([]SupportTicket, 0, len(rows))
	for _, row := range rows {
		name, email := "", ""
		if u := users[row.UserID]; u != nil {
			name = firstNonEmpty(u.Username, u.Email)
			email = u.Email
		}
		out = append(out, ticketToView(row, name, email))
	}
	return out, total, nil
}

func (s *TicketService) listMessages(ctx context.Context, ticketID int64) ([]SupportTicketMessage, error) {
	rows, err := s.entClient.SupportTicketMessage.Query().
		Where(supportticketmessage.TicketIDEQ(ticketID)).
		Order(supportticketmessage.ByCreatedAt(), supportticketmessage.ByID()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]SupportTicketMessage, 0, len(rows))
	for _, row := range rows {
		out = append(out, ticketMessageToView(row))
	}
	return out, nil
}

func ticketToView(row *dbent.SupportTicket, userName, userEmail string) SupportTicket {
	return SupportTicket{
		ID:                 row.ID,
		TicketNo:           row.TicketNo,
		UserID:             row.UserID,
		UserName:           userName,
		UserEmail:          userEmail,
		Category:           row.Category,
		Title:              row.Title,
		Status:             row.Status,
		CurrentFormPayload: row.CurrentFormPayload,
		CurrentRevisionNo:  row.CurrentRevisionNo,
		LatestMessageAt:    row.LatestMessageAt,
		LastReplyRole:      row.LastReplyRole,
		UnreadByUser:       row.UnreadByUser,
		UnreadByAdmin:      row.UnreadByAdmin,
		SubmittedAt:        row.SubmittedAt,
		ClosedAt:           row.ClosedAt,
		WithdrawnAt:        row.WithdrawnAt,
		CreatedAt:          row.CreatedAt,
		UpdatedAt:          row.UpdatedAt,
	}
}

func ticketMessageToView(row *dbent.SupportTicketMessage) SupportTicketMessage {
	var attachments []TicketMessageAttachment
	if len(row.Attachments) > 0 {
		_ = json.Unmarshal(row.Attachments, &attachments)
	}
	return SupportTicketMessage{
		ID:                   row.ID,
		TicketID:             row.TicketID,
		SenderRole:           row.SenderRole,
		SenderUserID:         row.SenderUserID,
		SenderNameSnapshot:   row.SenderNameSnapshot,
		SenderAvatarSnapshot: row.SenderAvatarSnapshot,
		MessageType:          row.MessageType,
		Content:              row.Content,
		Attachments:          attachments,
		CreatedAt:            row.CreatedAt,
	}
}

func ticketTemplateToView(row *dbent.SupportTicketReplyTemplate) TicketReplyTemplate {
	return TicketReplyTemplate{
		ID:        row.ID,
		Title:     row.Title,
		Content:   row.Content,
		SortOrder: row.SortOrder,
		CreatedAt: row.CreatedAt,
		UpdatedAt: row.UpdatedAt,
	}
}

func (s *TicketService) loadTicketUserAvatarURLBestEffort(ctx context.Context, userID int64) string {
	if s == nil || s.entClient == nil || userID <= 0 {
		return ""
	}
	rows, err := s.entClient.QueryContext(ctx, `SELECT url FROM user_avatars WHERE user_id = $1`, userID)
	if err != nil {
		return ""
	}
	defer func() { _ = rows.Close() }()
	if !rows.Next() {
		return ""
	}
	var avatarURL string
	if err := rows.Scan(&avatarURL); err != nil {
		return ""
	}
	return strings.TrimSpace(avatarURL)
}

func insertTicketSystemMessage(ctx context.Context, client *dbent.Client, ticketID int64, content string, at time.Time) error {
	_, err := client.SupportTicketMessage.Create().
		SetTicketID(ticketID).
		SetSenderRole(SupportTicketSenderRoleSystem).
		SetSenderNameSnapshot("系统").
		SetMessageType(SupportTicketMessageTypeSystem).
		SetContent(content).
		SetAttachments([]byte("[]")).
		SetCreatedAt(at).
		Save(ctx)
	return err
}

func isRecentDuplicateTicketReply(ctx context.Context, client *dbent.Client, ticketID int64, message *SupportTicketMessage) (bool, error) {
	last, err := client.SupportTicketMessage.Query().
		Where(supportticketmessage.TicketIDEQ(ticketID)).
		Order(dbent.Desc(supportticketmessage.FieldCreatedAt), dbent.Desc(supportticketmessage.FieldID)).
		First(ctx)
	if err != nil {
		if dbent.IsNotFound(err) {
			return false, nil
		}
		return false, err
	}
	if message.CreatedAt.Sub(last.CreatedAt) > supportTicketReplyDedupWindow || message.CreatedAt.Before(last.CreatedAt) {
		return false, nil
	}
	if last.SenderRole != message.SenderRole || last.MessageType != message.MessageType || last.Content != message.Content {
		return false, nil
	}
	if (last.SenderUserID == nil) != (message.SenderUserID == nil) {
		return false, nil
	}
	if last.SenderUserID != nil && message.SenderUserID != nil && *last.SenderUserID != *message.SenderUserID {
		return false, nil
	}
	var existing []TicketMessageAttachment
	_ = json.Unmarshal(last.Attachments, &existing)
	want, _ := json.Marshal(message.Attachments)
	have, _ := json.Marshal(existing)
	return string(want) == string(have), nil
}

func (s *TicketService) prepareTicketPayload(ctx context.Context, userID int64, category string, payload json.RawMessage) (json.RawMessage, error) {
	normalized := normalizeTicketPayload(payload)
	if len(normalized) == 0 {
		return nil, ErrTicketPayloadRequired
	}
	if err := validateSupportTicketPayloadShape(normalized); err != nil {
		return nil, err
	}
	if err := validateSupportTicketPayload(category, normalized); err != nil {
		return nil, err
	}
	if category != SupportTicketCategoryRateApply {
		return normalized, nil
	}
	return s.snapshotRateApplyPayload(ctx, userID, normalized)
}

func (s *TicketService) snapshotRateApplyPayload(ctx context.Context, userID int64, payload json.RawMessage) (json.RawMessage, error) {
	var form map[string]any
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	if err := decoder.Decode(&form); err != nil {
		return nil, ErrTicketPayloadInvalid
	}
	groupIDs, err := parseTicketGroupIDs(form["group_ids"])
	if err != nil {
		return nil, err
	}
	options, err := s.ListRateApplyGroups(ctx, userID)
	if err != nil {
		return nil, err
	}
	byID := make(map[int64]TicketRateGroupOption, len(options))
	for _, opt := range options {
		byID[opt.GroupID] = opt
	}
	effective := make(map[string]float64, len(groupIDs))
	snapshots := make([]map[string]any, 0, len(groupIDs))
	for _, id := range groupIDs {
		opt, ok := byID[id]
		if !ok {
			return nil, ErrTicketPayloadInvalid
		}
		key := strconv.FormatInt(id, 10)
		effective[key] = opt.EffectiveRate
		snapshots = append(snapshots, map[string]any{
			"group_id":             opt.GroupID,
			"name":                 opt.Name,
			"base_rate_multiplier": opt.BaseRateMultiplier,
			"effective_rate":       opt.EffectiveRate,
		})
	}
	form["group_ids"] = groupIDs
	form["effective_rates"] = effective
	form["group_snapshots"] = snapshots
	out, err := json.Marshal(form)
	if err != nil {
		return nil, ErrTicketPayloadInvalid
	}
	return out, nil
}

func parseTicketGroupIDs(raw any) ([]int64, error) {
	items, ok := raw.([]any)
	if !ok || len(items) == 0 || len(items) > supportTicketPayloadMaxFields {
		return nil, ErrTicketPayloadInvalid
	}
	out := make([]int64, 0, len(items))
	seen := map[int64]struct{}{}
	for _, item := range items {
		id, err := parsePositiveTicketInt64(item)
		if err != nil {
			return nil, err
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	if len(out) == 0 {
		return nil, ErrTicketPayloadInvalid
	}
	return out, nil
}

func parsePositiveTicketInt64(value any) (int64, error) {
	switch typed := value.(type) {
	case json.Number:
		id, err := typed.Int64()
		if err != nil || id <= 0 {
			return 0, ErrTicketPayloadInvalid
		}
		return id, nil
	case float64:
		id := int64(typed)
		if float64(id) != typed || id <= 0 {
			return 0, ErrTicketPayloadInvalid
		}
		return id, nil
	default:
		return 0, ErrTicketPayloadInvalid
	}
}

func ticketSenderDisplayName(role, username, email string) string {
	if role == SupportTicketSenderRoleAdmin {
		name := strings.TrimSpace(username)
		if name != "" && !strings.Contains(name, "@") {
			return name
		}
		return supportTicketStaffDisplayName
	}
	return firstNonEmpty(username, email)
}

func normalizeSupportTicketListFilters(filters SupportTicketListFilters) (SupportTicketListFilters, error) {
	if raw := strings.TrimSpace(filters.Category); raw != "" {
		filters.Category = NormalizeSupportTicketCategory(raw)
		if filters.Category == "" {
			return filters, ErrTicketInvalidCategory
		}
	}
	if raw := strings.TrimSpace(filters.Status); raw != "" {
		filters.Status = NormalizeSupportTicketStatus(raw)
		if filters.Status == "" {
			return filters, ErrTicketInvalidStatus
		}
	}
	return filters, nil
}

func isAdminManualStatusTransitionAllowed(currentStatus, lastReplyRole, nextStatus string) bool {
	if currentStatus == nextStatus {
		return true
	}
	if currentStatus == SupportTicketStatusWithdrawn || currentStatus == SupportTicketStatusClosed {
		return false
	}
	if nextStatus == SupportTicketStatusWaitingUser || nextStatus == SupportTicketStatusResolved {
		return lastReplyRole == SupportTicketSenderRoleAdmin
	}
	return true
}

func queryTicketForUpdate(ctx context.Context, client *dbent.Client, ticketID int64) (*dbent.SupportTicket, error) {
	row, err := client.SupportTicket.Query().Where(supportticket.IDEQ(ticketID)).ForUpdate().Only(ctx)
	if err != nil && isSQLiteForUpdateUnsupported(err) {
		return client.SupportTicket.Get(ctx, ticketID)
	}
	return row, err
}

func adminTicketStatusMessage(status string) string {
	switch status {
	case SupportTicketStatusProcessing:
		return "管理员将工单状态更新为处理中。"
	case SupportTicketStatusWaitingUser:
		return "管理员将工单状态更新为待用户回复。"
	case SupportTicketStatusWaitingAdmin:
		return "管理员将工单状态更新为待管理员处理。"
	case SupportTicketStatusResolved:
		return "管理员已完结工单。"
	case SupportTicketStatusClosed:
		return "管理员关闭了工单。"
	default:
		return ""
	}
}

func normalizeTicketPayload(payload json.RawMessage) json.RawMessage {
	trimmed := strings.TrimSpace(string(payload))
	if trimmed == "" || trimmed == "null" || trimmed == "{}" {
		return nil
	}
	var dst any
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	if err := decoder.Decode(&dst); err != nil {
		return nil
	}
	normalized, err := json.Marshal(dst)
	if err != nil {
		return nil
	}
	return normalized
}

func normalizeSupportTicketTitle(title string) (string, error) {
	trimmed := strings.TrimSpace(title)
	if trimmed == "" || utf8.RuneCountInString(trimmed) > supportTicketTitleMaxRunes || len(trimmed) > supportTicketTitleMaxBytes {
		return "", ErrTicketInvalidTitle
	}
	return trimmed, nil
}

func validateSupportTicketReplyContent(content string, hasAttachments bool) error {
	if content == "" && !hasAttachments {
		return ErrTicketMessageRequired
	}
	if len(content) > supportTicketReplyMaxBytes {
		return ErrTicketMessageTooLarge
	}
	return nil
}

func validateSupportTicketPayloadShape(payload json.RawMessage) error {
	if len(payload) > supportTicketPayloadMaxBytes {
		return ErrTicketPayloadInvalid
	}
	var decoded any
	decoder := json.NewDecoder(strings.NewReader(string(payload)))
	decoder.UseNumber()
	if err := decoder.Decode(&decoded); err != nil {
		return ErrTicketPayloadInvalid
	}
	root, ok := decoded.(map[string]any)
	if !ok || len(root) == 0 || len(root) > supportTicketPayloadMaxFields {
		return ErrTicketPayloadInvalid
	}
	return validateSupportTicketPayloadValue(root, 1)
}

func validateSupportTicketPayloadValue(value any, depth int) error {
	if depth > supportTicketPayloadMaxDepth {
		return ErrTicketPayloadInvalid
	}
	switch typed := value.(type) {
	case map[string]any:
		if len(typed) > supportTicketPayloadMaxFields {
			return ErrTicketPayloadInvalid
		}
		for key, child := range typed {
			if strings.TrimSpace(key) == "" || len(key) > supportTicketPayloadMaxFieldName {
				return ErrTicketPayloadInvalid
			}
			if err := validateSupportTicketPayloadValue(child, depth+1); err != nil {
				return err
			}
		}
	case []any:
		if len(typed) > supportTicketPayloadMaxFields {
			return ErrTicketPayloadInvalid
		}
		for _, child := range typed {
			if err := validateSupportTicketPayloadValue(child, depth+1); err != nil {
				return err
			}
		}
	case string, json.Number, bool, nil:
		return nil
	default:
		return ErrTicketPayloadInvalid
	}
	return nil
}

func validateSupportTicketPayload(category string, payload json.RawMessage) error {
	requiredFieldsByCategory := map[string][]string{
		SupportTicketCategoryConsult:          {"question"},
		SupportTicketCategoryRefund:           {"order_no", "reason"},
		SupportTicketCategoryConcurrencyApply: {"current_concurrency", "target_concurrency", "usage_scenario"},
		SupportTicketCategoryRateApply:        {"target_rate", "usage_scenario"},
		SupportTicketCategoryOther:            {"details"},
	}
	var form map[string]any
	if err := json.Unmarshal(payload, &form); err != nil {
		return ErrTicketPayloadInvalid
	}
	if category == SupportTicketCategoryRateApply {
		groupIDs, ok := form["group_ids"].([]any)
		if !ok || len(groupIDs) == 0 {
			return ErrTicketPayloadInvalid
		}
	}
	for _, fieldName := range requiredFieldsByCategory[category] {
		value, ok := form[fieldName]
		if !ok || value == nil || strings.TrimSpace(fmt.Sprint(value)) == "" {
			return ErrTicketPayloadInvalid
		}
	}
	return nil
}

func isTicketReplyLocked(status string) bool {
	return status == SupportTicketStatusResolved || status == SupportTicketStatusClosed || status == SupportTicketStatusWithdrawn
}

func generateTicketNo(now time.Time) string {
	var suffix [8]byte
	if _, err := rand.Read(suffix[:]); err == nil {
		return fmt.Sprintf("TK%s%s", now.Format("20060102150405"), hex.EncodeToString(suffix[:]))
	}
	return fmt.Sprintf("TK%s%016x", now.Format("20060102150405"), uint64(now.UnixNano()))
}
