package service

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/pkg/pagination"
)

const (
	supportTicketTitleMaxRunes       = 80
	supportTicketTitleMaxBytes       = 240
	supportTicketPayloadMaxBytes     = 16 * 1024
	supportTicketPayloadMaxFields    = 32
	supportTicketPayloadMaxFieldName = 64
	supportTicketPayloadMaxDepth     = 4
	supportTicketReplyMaxBytes       = 8 * 1024
)

type TicketService struct {
	ticketRepo SupportTicketRepository
	userRepo   UserRepository
}

func NewTicketService(ticketRepo SupportTicketRepository, userRepo UserRepository) *TicketService {
	return &TicketService{
		ticketRepo: ticketRepo,
		userRepo:   userRepo,
	}
}

func (s *TicketService) Create(ctx context.Context, input CreateSupportTicketInput) (*SupportTicket, error) {
	category := NormalizeSupportTicketCategory(input.Category)
	if category == "" {
		return nil, ErrTicketInvalidCategory
	}
	title, err := normalizeSupportTicketTitle(input.Title)
	if err != nil {
		return nil, ErrTicketInvalidTitle
	}
	payload := normalizeTicketPayload(input.FormPayload)
	if len(payload) == 0 {
		return nil, ErrTicketPayloadRequired
	}
	if err := validateSupportTicketPayloadShape(payload); err != nil {
		return nil, err
	}
	if err := validateSupportTicketPayload(category, payload); err != nil {
		return nil, err
	}

	user, err := s.userRepo.GetByID(ctx, input.UserID)
	if err != nil {
		return nil, fmt.Errorf("get ticket user: %w", err)
	}
	avatarURL := s.loadTicketUserAvatarURLBestEffort(ctx, input.UserID)

	now := time.Now()
	ticket := &SupportTicket{
		UserID:             input.UserID,
		UserName:           firstNonEmptyTicketString(user.Username, user.Email),
		UserEmail:          user.Email,
		UserAvatarURL:      avatarURL,
		Category:           category,
		Title:              title,
		Status:             SupportTicketStatusSubmitted,
		CurrentFormPayload: payload,
		CurrentRevisionNo:  1,
		LatestMessageAt:    now,
		LastReplyRole:      SupportTicketSenderRoleSystem,
		UnreadByUser:       false,
		UnreadByAdmin:      true,
		SubmittedAt:        &now,
	}
	revision := &SupportTicketRevision{
		RevisionNo:  1,
		Title:       title,
		FormPayload: payload,
		SubmittedBy: &input.UserID,
		SubmittedAt: now,
	}
	systemMessage := newSystemTicketMessage("用户提交了工单。", now)

	if err := s.ticketRepo.CreateSubmitted(ctx, ticket, revision, systemMessage); err != nil {
		return nil, fmt.Errorf("create ticket: %w", err)
	}
	return s.ticketRepo.GetByID(ctx, ticket.ID)
}

func (s *TicketService) GetForUser(ctx context.Context, userID, ticketID int64) (*SupportTicket, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.UserID != userID {
		return nil, ErrTicketForbidden
	}
	return ticket, nil
}

func (s *TicketService) GetForAdmin(ctx context.Context, ticketID int64) (*SupportTicket, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	return ticket, nil
}

func (s *TicketService) ListForUser(ctx context.Context, userID int64, params pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error) {
	var err error
	filters, err = normalizeSupportTicketListFilters(filters)
	if err != nil {
		return nil, nil, err
	}
	return s.ticketRepo.ListForUser(ctx, userID, params, filters)
}

func (s *TicketService) ListForAdmin(ctx context.Context, params pagination.PaginationParams, filters SupportTicketListFilters) ([]SupportTicket, *pagination.PaginationResult, error) {
	var err error
	filters, err = normalizeSupportTicketListFilters(filters)
	if err != nil {
		return nil, nil, err
	}
	return s.ticketRepo.ListForAdmin(ctx, params, filters)
}

func (s *TicketService) ListMessagesForUser(ctx context.Context, userID, ticketID int64) ([]SupportTicketMessage, error) {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if ticket.UserID != userID {
		return nil, ErrTicketForbidden
	}
	items, err := s.ticketRepo.ListMessages(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if err := s.ticketRepo.MarkReadByUser(ctx, ticketID); err != nil {
		return nil, fmt.Errorf("mark ticket read by user: %w", err)
	}
	return items, nil
}

func (s *TicketService) ListMessagesForAdmin(ctx context.Context, ticketID int64) ([]SupportTicketMessage, error) {
	if _, err := s.ticketRepo.GetByID(ctx, ticketID); err != nil {
		return nil, err
	}
	items, err := s.ticketRepo.ListMessages(ctx, ticketID)
	if err != nil {
		return nil, err
	}
	if err := s.ticketRepo.MarkReadByAdmin(ctx, ticketID); err != nil {
		return nil, fmt.Errorf("mark ticket read by admin: %w", err)
	}
	return items, nil
}

func (s *TicketService) Withdraw(ctx context.Context, userID, ticketID int64) error {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
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
	return s.ticketRepo.UpdateAfterUserWithdraw(ctx, ticketID, now, newSystemTicketMessage("用户撤回了工单，进入可修改状态。", now))
}

func (s *TicketService) UpdateEditable(ctx context.Context, input UpdateSupportTicketInput, ticketID int64) error {
	if input.ExpectedRevisionNo <= 0 {
		return ErrTicketRevisionRequired
	}
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
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
		return ErrTicketInvalidTitle
	}
	payload := normalizeTicketPayload(input.FormPayload)
	if len(payload) == 0 {
		return ErrTicketPayloadRequired
	}
	if err := validateSupportTicketPayloadShape(payload); err != nil {
		return err
	}
	if err := validateSupportTicketPayload(ticket.Category, payload); err != nil {
		return err
	}
	return s.ticketRepo.UpdateEditableContent(ctx, ticketID, title, payload, input.ExpectedRevisionNo)
}

func (s *TicketService) Resubmit(ctx context.Context, input UpdateSupportTicketInput, ticketID int64) error {
	if input.ExpectedRevisionNo <= 0 {
		return ErrTicketRevisionRequired
	}
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
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
		return ErrTicketInvalidTitle
	}
	payload := normalizeTicketPayload(input.FormPayload)
	if len(payload) == 0 {
		return ErrTicketPayloadRequired
	}
	if err := validateSupportTicketPayloadShape(payload); err != nil {
		return err
	}
	if err := validateSupportTicketPayload(ticket.Category, payload); err != nil {
		return err
	}
	now := time.Now()
	submittedAt := now
	nextTicket := &SupportTicket{
		Title:              title,
		Status:             SupportTicketStatusSubmitted,
		CurrentFormPayload: payload,
		LatestMessageAt:    now,
		LastReplyRole:      SupportTicketSenderRoleSystem,
		UnreadByUser:       false,
		UnreadByAdmin:      true,
		SubmittedAt:        &submittedAt,
	}
	revision := &SupportTicketRevision{
		Title:       title,
		FormPayload: payload,
		SubmittedBy: &input.UserID,
		SubmittedAt: now,
	}
	return s.ticketRepo.Resubmit(ctx, ticketID, nextTicket, revision, newSystemTicketMessage("用户更新了工单内容并重新提交。", now), input.ExpectedRevisionNo)
}

func (s *TicketService) CloseForUser(ctx context.Context, userID, ticketID int64) error {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
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
	return s.ticketRepo.CloseByUser(ctx, ticketID, now, newSystemTicketMessage("用户关闭了工单。", now))
}

func (s *TicketService) ReplyForUser(ctx context.Context, ticketID int64, input CreateSupportTicketMessageInput) error {
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return err
	}
	if ticket.UserID != input.UserID {
		return ErrTicketForbidden
	}
	if isTicketReplyLocked(ticket.Status) {
		return ErrTicketReplyLocked
	}
	content := strings.TrimSpace(input.Content)
	if err := validateSupportTicketReplyContent(content, len(input.Attachments) > 0); err != nil {
		return err
	}
	user, err := s.userRepo.GetByID(ctx, input.UserID)
	if err != nil {
		return fmt.Errorf("get reply user: %w", err)
	}
	avatarURL := s.loadTicketUserAvatarURLBestEffort(ctx, input.UserID)
	now := time.Now()
	senderID := input.UserID
	return s.ticketRepo.AddReply(ctx, ticketID, &SupportTicketMessage{
		SenderRole:           SupportTicketSenderRoleUser,
		SenderUserID:         &senderID,
		SenderNameSnapshot:   firstNonEmptyTicketString(user.Username, user.Email),
		SenderAvatarSnapshot: avatarURL,
		MessageType:          SupportTicketMessageTypeMessage,
		Content:              content,
		Attachments:          input.Attachments,
		CreatedAt:            now,
	}, SupportTicketSenderRoleUser, false, true, SupportTicketStatusWaitingAdmin)
}

func (s *TicketService) ReplyForAdmin(ctx context.Context, ticketID int64, input CreateSupportTicketMessageInput) error {
	content := strings.TrimSpace(input.Content)
	if err := validateSupportTicketReplyContent(content, len(input.Attachments) > 0); err != nil {
		return err
	}
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return err
	}
	if isTicketReplyLocked(ticket.Status) {
		return ErrTicketReplyLocked
	}
	adminUser, err := s.userRepo.GetByID(ctx, input.UserID)
	if err != nil {
		return fmt.Errorf("get admin user: %w", err)
	}
	avatarURL := s.loadTicketUserAvatarURLBestEffort(ctx, input.UserID)
	now := time.Now()
	senderID := input.UserID
	return s.ticketRepo.AddReply(ctx, ticketID, &SupportTicketMessage{
		SenderRole:           SupportTicketSenderRoleAdmin,
		SenderUserID:         &senderID,
		SenderNameSnapshot:   firstNonEmptyTicketString(adminUser.Username, adminUser.Email),
		SenderAvatarSnapshot: avatarURL,
		MessageType:          SupportTicketMessageTypeMessage,
		Content:              content,
		Attachments:          input.Attachments,
		CreatedAt:            now,
	}, SupportTicketSenderRoleAdmin, true, false, SupportTicketStatusWaitingUser)
}

func (s *TicketService) UpdateStatusByAdmin(ctx context.Context, ticketID int64, input AdminSupportTicketStatusUpdateInput) error {
	status := NormalizeSupportTicketStatus(input.Status)
	if status == "" {
		return ErrTicketInvalidStatus
	}
	ticket, err := s.ticketRepo.GetByID(ctx, ticketID)
	if err != nil {
		return err
	}
	if ticket.Status == SupportTicketStatusClosed && status != SupportTicketStatusClosed {
		return ErrTicketStatusLocked
	}
	if ticket.Status == SupportTicketStatusResolved && status != SupportTicketStatusResolved && status != SupportTicketStatusClosed {
		return ErrTicketStatusLocked
	}
	if !isAdminManualStatusTransitionAllowed(ticket.Status, ticket.LastReplyRole, status) {
		return ErrTicketStatusInvalidTransition
	}
	var content string
	switch status {
	case SupportTicketStatusProcessing:
		content = "管理员将工单状态更新为处理中。"
	case SupportTicketStatusWaitingUser:
		content = "管理员将工单状态更新为待用户回复。"
	case SupportTicketStatusWaitingAdmin:
		content = "管理员将工单状态更新为待管理员处理。"
	case SupportTicketStatusResolved:
		content = "管理员已完结工单。"
	case SupportTicketStatusClosed:
		content = "管理员关闭了工单。"
	default:
		return ErrTicketInvalidStatus
	}
	now := time.Now()
	var closedAt *time.Time
	if status == SupportTicketStatusResolved || status == SupportTicketStatusClosed {
		closedAt = &now
	}
	return s.ticketRepo.UpdateStatusByAdmin(ctx, ticketID, status, closedAt, newSystemTicketMessage(content, now))
}

func normalizeSupportTicketListFilters(filters SupportTicketListFilters) (SupportTicketListFilters, error) {
	categoryInput := strings.TrimSpace(filters.Category)
	if categoryInput != "" {
		filters.Category = NormalizeSupportTicketCategory(categoryInput)
		if filters.Category == "" {
			return filters, ErrTicketInvalidCategory
		}
	} else {
		filters.Category = ""
	}
	statusInput := strings.TrimSpace(filters.Status)
	if statusInput != "" {
		filters.Status = NormalizeSupportTicketStatus(statusInput)
		if filters.Status == "" {
			return filters, ErrTicketInvalidStatus
		}
	} else {
		filters.Status = ""
	}
	return filters, nil
}

func isAdminManualStatusTransitionAllowed(currentStatus, lastReplyRole, nextStatus string) bool {
	if currentStatus == nextStatus {
		return true
	}
	if nextStatus == SupportTicketStatusWaitingUser || nextStatus == SupportTicketStatusResolved {
		return lastReplyRole == SupportTicketSenderRoleAdmin
	}
	return true
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

func newSystemTicketMessage(content string, createdAt time.Time) *SupportTicketMessage {
	return &SupportTicketMessage{
		SenderRole:           SupportTicketSenderRoleSystem,
		SenderNameSnapshot:   "系统",
		SenderAvatarSnapshot: "",
		MessageType:          SupportTicketMessageTypeSystem,
		Content:              content,
		CreatedAt:            createdAt,
	}
}

func firstNonEmptyTicketString(values ...string) string {
	for _, v := range values {
		if strings.TrimSpace(v) != "" {
			return strings.TrimSpace(v)
		}
	}
	return ""
}

func (s *TicketService) loadTicketUserAvatarURL(ctx context.Context, userID int64) (string, error) {
	avatar, err := s.userRepo.GetUserAvatar(ctx, userID)
	if err != nil {
		return "", err
	}
	if avatar == nil {
		return "", nil
	}
	return strings.TrimSpace(avatar.URL), nil
}

func (s *TicketService) loadTicketUserAvatarURLBestEffort(ctx context.Context, userID int64) string {
	avatarURL, err := s.loadTicketUserAvatarURL(ctx, userID)
	if err != nil {
		slog.WarnContext(ctx, "support ticket avatar snapshot lookup failed", "user_id", userID, "error", err)
		return ""
	}
	return avatarURL
}

func validateSupportTicketPayload(category string, payload json.RawMessage) error {
	requiredFieldsByCategory := map[string][]string{
		SupportTicketCategoryConsult:          {"question"},
		SupportTicketCategoryRefund:           {"order_no", "reason"},
		SupportTicketCategoryConcurrencyApply: {"current_concurrency", "target_concurrency", "usage_scenario"},
		SupportTicketCategoryRateApply:        {"target_rate", "usage_scenario"},
		SupportTicketCategoryOther:            {"details"},
	}
	requiredFields := requiredFieldsByCategory[category]
	if len(requiredFields) == 0 {
		return nil
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

	for _, field := range requiredFields {
		value, ok := form[field]
		if !ok {
			return ErrTicketPayloadInvalid
		}
		if strings.TrimSpace(fmt.Sprint(value)) == "" || value == nil {
			return ErrTicketPayloadInvalid
		}
	}
	return nil
}

func isTicketReplyLocked(status string) bool {
	return status == SupportTicketStatusResolved || status == SupportTicketStatusClosed || status == SupportTicketStatusWithdrawn
}
