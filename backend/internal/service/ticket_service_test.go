//go:build unit

package service

import (
	"context"
	"encoding/json"
	"strconv"
	"testing"

	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
	"github.com/stretchr/testify/require"
)

func TestTicketCreateRequiresCategoryPayloadAndSupportsWithdrawResubmit(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewTicketService(client, nil)
	userID := createInvoiceTestUser(t, client, "ticket@example.com")

	_, err := svc.Create(ctx, CreateSupportTicketInput{
		UserID: userID, Category: "nope", Title: "Help", FormPayload: json.RawMessage(`{"question":"x"}`),
	})
	require.Equal(t, "TICKET_CATEGORY_INVALID", infraerrors.Reason(err))

	_, err = svc.Create(ctx, CreateSupportTicketInput{
		UserID: userID, Category: SupportTicketCategoryConsult, Title: "Help", FormPayload: json.RawMessage(`{}`),
	})
	require.Equal(t, "TICKET_PAYLOAD_REQUIRED", infraerrors.Reason(err))

	created, err := svc.Create(ctx, CreateSupportTicketInput{
		UserID:      userID,
		Category:    SupportTicketCategoryConsult,
		Title:       "Cannot login",
		FormPayload: json.RawMessage(`{"question":"reset password"}`),
	})
	require.NoError(t, err)
	require.Equal(t, SupportTicketStatusSubmitted, created.Status)
	require.Equal(t, 1, created.CurrentRevisionNo)
	require.True(t, created.UnreadByAdmin)
	require.NotEmpty(t, created.TicketNo)

	require.NoError(t, svc.Withdraw(ctx, userID, created.ID))
	withdrawn, err := svc.GetForUser(ctx, userID, created.ID)
	require.NoError(t, err)
	require.Equal(t, SupportTicketStatusWithdrawn, withdrawn.Status)

	err = svc.Resubmit(ctx, created.ID, UpdateSupportTicketInput{
		UserID: userID, Title: "Cannot login v2", FormPayload: json.RawMessage(`{"question":"still locked"}`), ExpectedRevisionNo: 99,
	})
	require.Equal(t, "TICKET_REVISION_CONFLICT", infraerrors.Reason(err))

	require.NoError(t, svc.Resubmit(ctx, created.ID, UpdateSupportTicketInput{
		UserID:             userID,
		Title:              "Cannot login v2",
		FormPayload:        json.RawMessage(`{"question":"still locked"}`),
		ExpectedRevisionNo: withdrawn.CurrentRevisionNo,
	}))
	resubmitted, err := svc.GetForUser(ctx, userID, created.ID)
	require.NoError(t, err)
	require.Equal(t, SupportTicketStatusSubmitted, resubmitted.Status)
	require.Equal(t, 2, resubmitted.CurrentRevisionNo)
}

func TestTicketReplyDedupAndAdminStatusRules(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewTicketService(client, nil)
	userID := createInvoiceTestUser(t, client, "user-ticket@example.com")
	adminID := createInvoiceTestUser(t, client, "admin-ticket@example.com")
	_, err := client.User.UpdateOneID(adminID).SetRole("admin").SetUsername("").Save(ctx)
	require.NoError(t, err)

	ticket, err := svc.Create(ctx, CreateSupportTicketInput{
		UserID: userID, Category: SupportTicketCategoryOther, Title: "Other", FormPayload: json.RawMessage(`{"details":"please help"}`),
	})
	require.NoError(t, err)

	require.NoError(t, svc.ReplyForUser(ctx, ticket.ID, CreateSupportTicketMessageInput{UserID: userID, Content: "more info"}))
	require.NoError(t, svc.ReplyForUser(ctx, ticket.ID, CreateSupportTicketMessageInput{UserID: userID, Content: "more info"}))
	msgs, err := svc.ListMessagesForUser(ctx, userID, ticket.ID)
	require.NoError(t, err)
	userReplies := 0
	for _, msg := range msgs {
		if msg.SenderRole == SupportTicketSenderRoleUser && msg.Content == "more info" {
			userReplies++
		}
	}
	require.Equal(t, 1, userReplies)

	err = svc.UpdateStatusByAdmin(ctx, ticket.ID, SupportTicketStatusResolved)
	require.Equal(t, "TICKET_STATUS_TRANSITION_INVALID", infraerrors.Reason(err))

	require.NoError(t, svc.ReplyForAdmin(ctx, ticket.ID, CreateSupportTicketMessageInput{UserID: adminID, Content: "we are looking"}))
	adminMsgs, err := svc.ListMessagesForAdmin(ctx, ticket.ID)
	require.NoError(t, err)
	var staffName string
	for _, msg := range adminMsgs {
		if msg.SenderRole == SupportTicketSenderRoleAdmin {
			staffName = msg.SenderNameSnapshot
		}
	}
	require.Equal(t, supportTicketStaffDisplayName, staffName)
	userMsgs, err := svc.ListMessagesForUser(ctx, userID, ticket.ID)
	require.NoError(t, err)
	for _, msg := range userMsgs {
		if msg.SenderRole != SupportTicketSenderRoleUser {
			require.Nil(t, msg.SenderUserID)
		}
	}
	require.NoError(t, svc.UpdateStatusByAdmin(ctx, ticket.ID, SupportTicketStatusProcessing))
	afterProcessing, err := svc.GetForAdmin(ctx, ticket.ID)
	require.NoError(t, err)
	require.Equal(t, SupportTicketSenderRoleAdmin, afterProcessing.LastReplyRole)
	require.True(t, afterProcessing.UnreadByUser)
	require.NoError(t, svc.UpdateStatusByAdmin(ctx, ticket.ID, SupportTicketStatusResolved))
	resolved, err := svc.GetForAdmin(ctx, ticket.ID)
	require.NoError(t, err)
	require.Equal(t, SupportTicketStatusResolved, resolved.Status)
	require.Equal(t, SupportTicketSenderRoleAdmin, resolved.LastReplyRole)

	err = svc.ReplyForUser(ctx, ticket.ID, CreateSupportTicketMessageInput{UserID: userID, Content: "thanks"})
	require.Equal(t, "TICKET_REPLY_LOCKED", infraerrors.Reason(err))
}

func TestTicketEditIncrementsRevisionAndBlocksWithdrawnAdminTransition(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewTicketService(client, nil)
	userID := createInvoiceTestUser(t, client, "edit@example.com")
	ticket, err := svc.Create(ctx, CreateSupportTicketInput{
		UserID: userID, Category: SupportTicketCategoryConsult, Title: "Edit me",
		FormPayload: json.RawMessage(`{"question":"first"}`),
	})
	require.NoError(t, err)
	require.NoError(t, svc.Withdraw(ctx, userID, ticket.ID))

	err = svc.UpdateStatusByAdmin(ctx, ticket.ID, SupportTicketStatusProcessing)
	require.Equal(t, "TICKET_STATUS_TRANSITION_INVALID", infraerrors.Reason(err))

	require.NoError(t, svc.UpdateEditable(ctx, ticket.ID, UpdateSupportTicketInput{
		UserID: userID, Title: "Edit me 2", FormPayload: json.RawMessage(`{"question":"second"}`), ExpectedRevisionNo: 1,
	}))
	err = svc.UpdateEditable(ctx, ticket.ID, UpdateSupportTicketInput{
		UserID: userID, Title: "Edit me 3", FormPayload: json.RawMessage(`{"question":"third"}`), ExpectedRevisionNo: 1,
	})
	require.Equal(t, "TICKET_REVISION_CONFLICT", infraerrors.Reason(err))
	require.NoError(t, svc.Resubmit(ctx, ticket.ID, UpdateSupportTicketInput{
		UserID: userID, Title: "Edit me 2", FormPayload: json.RawMessage(`{"question":"second"}`), ExpectedRevisionNo: 2,
	}))
	reloaded, err := svc.GetForUser(ctx, userID, ticket.ID)
	require.NoError(t, err)
	require.Equal(t, 3, reloaded.CurrentRevisionNo)
	require.Equal(t, SupportTicketStatusSubmitted, reloaded.Status)
}

func TestTicketRateApplyRequiresGroups(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewTicketService(client, nil)
	userID := createInvoiceTestUser(t, client, "rate@example.com")
	_, err := svc.Create(ctx, CreateSupportTicketInput{
		UserID: userID, Category: SupportTicketCategoryRateApply, Title: "Lower rate",
		FormPayload: json.RawMessage(`{"target_rate":"0.5","usage_scenario":"batch"}`),
	})
	require.Equal(t, "TICKET_PAYLOAD_INVALID", infraerrors.Reason(err))

	_, err = svc.Create(ctx, CreateSupportTicketInput{
		UserID: userID, Category: SupportTicketCategoryRateApply, Title: "Lower rate",
		FormPayload: json.RawMessage(`{"group_ids":[1],"target_rate":"0.5","usage_scenario":"batch","effective_rates":{"1":0.8}}`),
	})
	require.Equal(t, "TICKET_PAYLOAD_INVALID", infraerrors.Reason(err))

	group, err := client.Group.Create().
		SetName("standard-a").
		SetRateMultiplier(1.25).
		SetStatus("active").
		SetSubscriptionType("standard").
		Save(ctx)
	require.NoError(t, err)
	payload, err := json.Marshal(map[string]any{
		"group_ids":       []int64{group.ID},
		"target_rate":     "0.5",
		"usage_scenario":  "batch",
		"effective_rates": map[string]float64{strconv.FormatInt(group.ID, 10): 0.8},
	})
	require.NoError(t, err)
	created, err := svc.Create(ctx, CreateSupportTicketInput{
		UserID: userID, Category: SupportTicketCategoryRateApply, Title: "Lower rate", FormPayload: payload,
	})
	require.NoError(t, err)
	var stored map[string]any
	require.NoError(t, json.Unmarshal(created.CurrentFormPayload, &stored))
	rates, ok := stored["effective_rates"].(map[string]any)
	require.True(t, ok)
	require.Equal(t, 1.25, rates[strconv.FormatInt(group.ID, 10)])
}

func TestTicketAttachmentMustBelongToSameTicket(t *testing.T) {
	ctx := context.Background()
	client := newPaymentConfigServiceTestClient(t)
	svc := NewTicketService(client, nil)
	userID := createInvoiceTestUser(t, client, "attach@example.com")
	ticket, err := svc.Create(ctx, CreateSupportTicketInput{
		UserID: userID, Category: SupportTicketCategoryOther, Title: "Files",
		FormPayload: json.RawMessage(`{"details":"see file"}`),
	})
	require.NoError(t, err)
	other, err := svc.Create(ctx, CreateSupportTicketInput{
		UserID: userID, Category: SupportTicketCategoryOther, Title: "Other files",
		FormPayload: json.RawMessage(`{"details":"other"}`),
	})
	require.NoError(t, err)
	foreign := createInvoiceTestMedia(t, client, userID, other.ID)
	_, err = client.MediaAsset.UpdateOneID(foreign).SetBizType(MediaBizTicket).Save(ctx)
	require.NoError(t, err)
	err = svc.ReplyForUser(ctx, ticket.ID, CreateSupportTicketMessageInput{UserID: userID, MediaIDs: []int64{foreign}})
	require.Equal(t, "TICKET_ATTACHMENT_INVALID", infraerrors.Reason(err))
	err = svc.EnsureTicketAttachmentAccess(ctx, ticket.ID, foreign, userID, false)
	require.Equal(t, "TICKET_ATTACHMENT_INVALID", infraerrors.Reason(err))

	tooMany := make([]int64, supportTicketMaxAttachments+1)
	for i := range tooMany {
		tooMany[i] = int64(i + 1)
	}
	err = svc.ReplyForUser(ctx, ticket.ID, CreateSupportTicketMessageInput{UserID: userID, MediaIDs: tooMany})
	require.Equal(t, "TICKET_ATTACHMENT_LIMIT", infraerrors.Reason(err))
}
