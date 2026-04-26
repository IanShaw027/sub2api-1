package dto

import (
	"encoding/json"
	"time"

	"github.com/Wei-Shaw/sub2api/internal/service"
)

type SupportTicket struct {
	ID                 int64           `json:"id"`
	TicketNo           string          `json:"ticket_no"`
	UserID             int64           `json:"user_id"`
	UserName           string          `json:"user_name"`
	UserEmail          string          `json:"user_email"`
	UserAvatarURL      string          `json:"user_avatar_url"`
	Category           string          `json:"category"`
	Title              string          `json:"title"`
	Status             string          `json:"status"`
	CurrentFormPayload json.RawMessage `json:"current_form_payload"`
	CurrentRevisionNo  int             `json:"current_revision_no"`
	LatestMessageAt    time.Time       `json:"latest_message_at"`
	LastReplyRole      string          `json:"last_reply_role"`
	UnreadByUser       bool            `json:"unread_by_user"`
	UnreadByAdmin      bool            `json:"unread_by_admin"`
	SubmittedAt        *time.Time      `json:"submitted_at,omitempty"`
	ClosedAt           *time.Time      `json:"closed_at,omitempty"`
	WithdrawnAt        *time.Time      `json:"withdrawn_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type UserSupportTicket struct {
	ID                 int64           `json:"id"`
	TicketNo           string          `json:"ticket_no"`
	Category           string          `json:"category"`
	Title              string          `json:"title"`
	Status             string          `json:"status"`
	CurrentFormPayload json.RawMessage `json:"current_form_payload"`
	CurrentRevisionNo  int             `json:"current_revision_no"`
	LatestMessageAt    time.Time       `json:"latest_message_at"`
	LastReplyRole      string          `json:"last_reply_role"`
	UnreadByUser       bool            `json:"unread_by_user"`
	UnreadByAdmin      bool            `json:"unread_by_admin"`
	SubmittedAt        *time.Time      `json:"submitted_at,omitempty"`
	ClosedAt           *time.Time      `json:"closed_at,omitempty"`
	WithdrawnAt        *time.Time      `json:"withdrawn_at,omitempty"`
	CreatedAt          time.Time       `json:"created_at"`
	UpdatedAt          time.Time       `json:"updated_at"`
}

type SupportTicketMessage struct {
	ID                   int64     `json:"id"`
	TicketID             int64     `json:"ticket_id"`
	SenderRole           string    `json:"sender_role"`
	SenderUserID         *int64    `json:"sender_user_id,omitempty"`
	SenderNameSnapshot   string    `json:"sender_name_snapshot"`
	SenderAvatarSnapshot string    `json:"sender_avatar_snapshot"`
	MessageType          string    `json:"message_type"`
	Content              string    `json:"content"`
	CreatedAt            time.Time `json:"created_at"`
}

func SupportTicketFromService(item *service.SupportTicket) *SupportTicket {
	if item == nil {
		return nil
	}
	return &SupportTicket{
		ID:                 item.ID,
		TicketNo:           item.TicketNo,
		UserID:             item.UserID,
		UserName:           item.UserName,
		UserEmail:          item.UserEmail,
		UserAvatarURL:      item.UserAvatarURL,
		Category:           item.Category,
		Title:              item.Title,
		Status:             item.Status,
		CurrentFormPayload: item.CurrentFormPayload,
		CurrentRevisionNo:  item.CurrentRevisionNo,
		LatestMessageAt:    item.LatestMessageAt,
		LastReplyRole:      item.LastReplyRole,
		UnreadByUser:       item.UnreadByUser,
		UnreadByAdmin:      item.UnreadByAdmin,
		SubmittedAt:        item.SubmittedAt,
		ClosedAt:           item.ClosedAt,
		WithdrawnAt:        item.WithdrawnAt,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
}

func UserSupportTicketFromService(item *service.SupportTicket) *UserSupportTicket {
	if item == nil {
		return nil
	}
	return &UserSupportTicket{
		ID:                 item.ID,
		TicketNo:           item.TicketNo,
		Category:           item.Category,
		Title:              item.Title,
		Status:             item.Status,
		CurrentFormPayload: item.CurrentFormPayload,
		CurrentRevisionNo:  item.CurrentRevisionNo,
		LatestMessageAt:    item.LatestMessageAt,
		LastReplyRole:      item.LastReplyRole,
		UnreadByUser:       item.UnreadByUser,
		UnreadByAdmin:      item.UnreadByAdmin,
		SubmittedAt:        item.SubmittedAt,
		ClosedAt:           item.ClosedAt,
		WithdrawnAt:        item.WithdrawnAt,
		CreatedAt:          item.CreatedAt,
		UpdatedAt:          item.UpdatedAt,
	}
}

func SupportTicketMessageFromService(item *service.SupportTicketMessage) *SupportTicketMessage {
	if item == nil {
		return nil
	}
	return &SupportTicketMessage{
		ID:                   item.ID,
		TicketID:             item.TicketID,
		SenderRole:           item.SenderRole,
		SenderUserID:         item.SenderUserID,
		SenderNameSnapshot:   item.SenderNameSnapshot,
		SenderAvatarSnapshot: item.SenderAvatarSnapshot,
		MessageType:          item.MessageType,
		Content:              item.Content,
		CreatedAt:            item.CreatedAt,
	}
}
