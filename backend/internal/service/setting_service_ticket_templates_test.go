package service

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/stretchr/testify/require"
)

type ticketTemplateSettingRepoStub struct {
	value   string
	setKey  string
	setCall int
}

func (*ticketTemplateSettingRepoStub) Get(context.Context, string) (*Setting, error) {
	panic("unexpected Get call")
}

func (s *ticketTemplateSettingRepoStub) GetValue(_ context.Context, key string) (string, error) {
	if key != SettingKeyAdminTicketReplyTemplates {
		panic("unexpected GetValue call")
	}
	if s.value == "" {
		return "", ErrSettingNotFound
	}
	return s.value, nil
}

func (*ticketTemplateSettingRepoStub) Set(context.Context, string, string) error {
	panic("unexpected Set call")
}

func (*ticketTemplateSettingRepoStub) GetMultiple(context.Context, []string) (map[string]string, error) {
	panic("unexpected GetMultiple call")
}

func (*ticketTemplateSettingRepoStub) SetMultiple(context.Context, map[string]string) error {
	panic("unexpected SetMultiple call")
}

func (*ticketTemplateSettingRepoStub) GetAll(context.Context) (map[string]string, error) {
	panic("unexpected GetAll call")
}

func (*ticketTemplateSettingRepoStub) Delete(context.Context, string) error {
	panic("unexpected Delete call")
}

type ticketTemplateSettingReadWriteRepoStub struct {
	ticketTemplateSettingRepoStub
}

func (s *ticketTemplateSettingReadWriteRepoStub) Set(_ context.Context, key, value string) error {
	if key != SettingKeyAdminTicketReplyTemplates {
		panic("unexpected Set call")
	}
	s.value = value
	s.setKey = key
	s.setCall++
	return nil
}

func TestSettingServiceGetAdminTicketReplyTemplatesStabilizesStoredDataWithoutReordering(t *testing.T) {
	repo := &ticketTemplateSettingRepoStub{
		value: `[
			{"id":"dup","title":" Beta ","content":" second "},
			{"id":"dup","title":"Alpha","content":" first "},
			{"title":"  No ID  ","content":" fallback "}
		]`,
	}
	svc := NewSettingService(repo, nil)

	first, err := svc.GetAdminTicketReplyTemplates(context.Background())
	require.NoError(t, err)

	second, err := svc.GetAdminTicketReplyTemplates(context.Background())
	require.NoError(t, err)

	require.Equal(t, first, second)
	require.Len(t, first, 3)
	require.Equal(t, "Beta", first[0].Title)
	require.Equal(t, "second", first[0].Content)
	require.Equal(t, "dup", first[0].ID)
	require.Equal(t, "Alpha", first[1].Title)
	require.Equal(t, "first", first[1].Content)
	require.Equal(t, "No ID", first[2].Title)
	require.NotEmpty(t, first[1].ID)
	require.NotEmpty(t, first[2].ID)
	require.NotEqual(t, first[0].ID, first[1].ID)
	require.NotEqual(t, first[1].ID, first[2].ID)
}

func TestSettingServiceGetAdminTicketReplyTemplates_NilRepoReturnsEmptyList(t *testing.T) {
	svc := NewSettingService(nil, nil)

	got, err := svc.GetAdminTicketReplyTemplates(context.Background())

	require.NoError(t, err)
	require.Empty(t, got)
}

func TestSettingServiceSetAdminTicketReplyTemplatesPreservesSubmittedOrder(t *testing.T) {
	repo := &ticketTemplateSettingReadWriteRepoStub{}
	svc := NewSettingService(repo, nil)

	input := []AdminTicketReplyTemplate{
		{ID: "beta", Title: " Beta ", Content: " second "},
		{ID: "alpha", Title: "Alpha", Content: " first "},
		{Title: "  No ID  ", Content: " fallback "},
	}

	err := svc.SetAdminTicketReplyTemplates(context.Background(), input)
	require.NoError(t, err)
	require.Equal(t, 1, repo.setCall)
	require.Equal(t, SettingKeyAdminTicketReplyTemplates, repo.setKey)

	var stored []AdminTicketReplyTemplate
	require.NoError(t, json.Unmarshal([]byte(repo.value), &stored))
	require.Len(t, stored, 3)
	require.Equal(t, "beta", stored[0].ID)
	require.Equal(t, "Beta", stored[0].Title)
	require.Equal(t, "second", stored[0].Content)
	require.Equal(t, "alpha", stored[1].ID)
	require.Equal(t, "Alpha", stored[1].Title)
	require.Equal(t, "first", stored[1].Content)
	require.NotEmpty(t, stored[2].ID)
	require.Equal(t, "No ID", stored[2].Title)
	require.Equal(t, "fallback", stored[2].Content)

	got, err := svc.GetAdminTicketReplyTemplates(context.Background())
	require.NoError(t, err)
	require.Equal(t, stored, got)
}

func TestSettingServiceSetAdminTicketReplyTemplates_NilRepoReturnsError(t *testing.T) {
	svc := NewSettingService(nil, nil)

	err := svc.SetAdminTicketReplyTemplates(context.Background(), []AdminTicketReplyTemplate{{Title: "Hi", Content: "There"}})

	require.Error(t, err)
	require.Contains(t, err.Error(), "setting repository unavailable")
}
