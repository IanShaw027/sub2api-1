package service

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"
	"unicode/utf8"

	"github.com/Wei-Shaw/sub2api/internal/domain"
	"github.com/google/uuid"
)

type CreationPublicationService struct {
	repo     CreationPublicationRepository
	resolver MediaStorageResolver
}

func NewCreationPublicationService(repo CreationPublicationRepository, resolver MediaStorageResolver) *CreationPublicationService {
	return &CreationPublicationService{repo: repo, resolver: resolver}
}

func publicationAccess(p *CreationPublication) *CreationPublication {
	if p != nil && p.WithdrawnAt == nil && p.Status == CreationPublicationPublished {
		p.MediaURL = fmt.Sprintf("/api/v1/creation/gallery/%d/media", p.ID)
	} else if p != nil {
		p.MediaURL = ""
	}
	return p
}

func publicationTextValid(value string, max int) bool {
	return utf8.ValidString(value) && utf8.RuneCountInString(value) <= max
}

func (s *CreationPublicationService) Publish(ctx context.Context, in PublishCreationInput) (*CreationPublication, error) {
	requestID, err := uuid.Parse(in.RequestID)
	in.Title = strings.TrimSpace(in.Title)
	in.Model = strings.TrimSpace(in.Model)
	if err != nil || in.OwnerUserID <= 0 || in.Visibility != MediaVisibilityPublic || in.Title == "" ||
		!publicationTextValid(in.Title, 160) || !publicationTextValid(in.Prompt, 16000) || !publicationTextValid(in.Model, 128) ||
		(in.Kind != "image" && in.Kind != "video") {
		return nil, ErrCreationPublicationInvalid
	}
	if len(in.Data) == 0 {
		return nil, ErrMediaEmptyUpload
	}
	if int64(len(in.Data)) > MaxCreationPublicationBytes {
		return nil, ErrMediaTooLarge
	}
	mime := sniffMediaMIME(in.Data)
	ext := map[string]string{"image/png": ".png", "image/jpeg": ".jpg", "image/gif": ".gif", "image/webp": ".webp", "video/mp4": ".mp4", "video/webm": ".webm"}[mime]
	if ext == "" || !strings.HasPrefix(mime, in.Kind+"/") {
		return nil, ErrMediaUnsupportedType
	}
	if in.Kind == "image" {
		if err := validateMediaImage(in.Data, mime); err != nil {
			return nil, err
		}
	}
	sum := sha256.Sum256(in.Data)
	p := &CreationPublication{
		OwnerUserID: in.OwnerUserID, RequestID: requestID.String(), Title: in.Title, Prompt: in.Prompt,
		Model: in.Model, Kind: in.Kind, MIME: mime, Size: int64(len(in.Data)), SHA256: hex.EncodeToString(sum[:]), Status: CreationPublicationPending,
	}
	existing, err := s.repo.GetByRequestID(ctx, p.OwnerUserID, p.RequestID)
	if err == nil {
		if _, err := matchingPublication(existing, p); err != nil {
			return nil, err
		}
		p = existing
	} else if errors.Is(err, ErrCreationPublicationNotFound) {
		binding, _, err := s.resolve(ctx, "")
		if err != nil {
			return nil, err
		}
		p.StorageProfileID = binding.ProfileID
		p.StorageKey = strings.TrimLeft(strings.Trim(binding.Prefix, "/")+"/creation-publications/"+uuid.NewString()+ext, "/")
		// Persist the upload intent before any external write. Even an ambiguous
		// INSERT response cannot orphan an object: no upload has started yet.
		stored, _, err := s.repo.CreateIfAbsent(ctx, p)
		if err != nil {
			return nil, err
		}
		if _, err := matchingPublication(stored, p); err != nil {
			return nil, err
		}
		p = stored
	} else {
		return nil, err
	}
	if p.Status == CreationPublicationPublished {
		return publicationAccess(p), nil
	}
	_, store, err := s.resolve(ctx, p.StorageProfileID)
	if err != nil {
		return nil, err
	}
	// Retries share the persisted key and identical content. Never delete a
	// pending key on upload failure: a concurrent retry may already be using it.
	if err := store.Put(ctx, p.StorageKey, p.MIME, in.Data); err != nil {
		return s.recoverPublicationUpload(ctx, p, store, err)
	}
	stored, err := s.repo.Activate(ctx, p.ID)
	if err != nil {
		return s.recoverPublicationUpload(ctx, p, store, err)
	}
	return publicationAccess(stored), nil
}

func (s *CreationPublicationService) recoverPublicationUpload(ctx context.Context, p *CreationPublication, store MediaObjectStore, uploadErr error) (*CreationPublication, error) {
	lookupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mediaObjectCleanupTimeout)
	defer cancel()
	stored, lookupErr := s.repo.GetByRequestID(lookupCtx, p.OwnerUserID, p.RequestID)
	if errors.Is(lookupErr, ErrCreationPublicationNotFound) {
		// The intent existed before Put, so a missing row now means it was
		// removed (for example with its owner), not an uncertain initial INSERT.
		return nil, errors.Join(uploadErr, cleanupPublicationObject(ctx, store, p.StorageKey))
	}
	if lookupErr != nil {
		// The pending row still records the key, so a later owner retry or DELETE
		// can recover or clean it once the database is available again.
		return nil, errors.Join(uploadErr, lookupErr)
	}
	if stored.WithdrawnAt != nil {
		// Withdrawal can finish while Put is in flight. Clean again after that
		// late Put; the immutable tombstone prevents any retry from publishing it.
		if err := cleanupPublicationObject(ctx, store, p.StorageKey); err != nil {
			return nil, errors.Join(ErrCreationPublicationConflict, err)
		}
		return nil, ErrCreationPublicationConflict
	}
	if stored.Status == CreationPublicationPublished {
		return matchingPublication(stored, p)
	}
	return nil, uploadErr
}

func matchingPublication(existing, candidate *CreationPublication) (*CreationPublication, error) {
	if existing.WithdrawnAt != nil || existing.SHA256 != candidate.SHA256 || existing.Title != candidate.Title ||
		existing.Prompt != candidate.Prompt || existing.Model != candidate.Model || existing.Kind != candidate.Kind {
		return nil, ErrCreationPublicationConflict
	}
	return publicationAccess(existing), nil
}

func cleanupPublicationObject(ctx context.Context, store MediaObjectStore, key string) error {
	cleanupCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), mediaObjectCleanupTimeout)
	defer cancel()
	return store.Delete(cleanupCtx, key)
}

func (s *CreationPublicationService) resolve(ctx context.Context, profileID string) (*MediaStorageBinding, MediaObjectStore, error) {
	if s.resolver == nil {
		return nil, nil, ErrMediaStorageNotConfigured
	}
	binding, store, err := s.resolver.Resolve(ctx)
	if err != nil {
		return nil, nil, err
	}
	if binding == nil || store == nil {
		return nil, nil, ErrMediaStorageNotConfigured
	}
	copy := *binding
	if copy.ProfileID == "" {
		copy.ProfileID = domain.MediaStorageProfileBackup
	}
	if profileID != "" && copy.ProfileID != profileID {
		return nil, nil, ErrMediaStorageNotConfigured
	}
	return &copy, store, nil
}

func (s *CreationPublicationService) Status(ctx context.Context, userID int64, requestID string) (*CreationPublication, error) {
	id, err := uuid.Parse(requestID)
	if err != nil {
		return nil, ErrCreationPublicationInvalid
	}
	p, err := s.repo.GetByRequestID(ctx, userID, id.String())
	if err != nil {
		return nil, err
	}
	return publicationAccess(p), nil
}

func (s *CreationPublicationService) List(ctx context.Context, filter CreationPublicationFilter) (*CreationPublicationPage, error) {
	filter.Search = strings.TrimSpace(filter.Search)
	if filter.Page < 1 || filter.Page > 1000000 || filter.PageSize < 1 || filter.PageSize > 60 ||
		(filter.Kind != "" && filter.Kind != "image" && filter.Kind != "video") || !publicationTextValid(filter.Search, 200) {
		return nil, ErrCreationPublicationInvalid
	}
	items, total, err := s.repo.List(ctx, filter)
	if err != nil {
		return nil, err
	}
	if items == nil {
		items = []*CreationPublication{}
	}
	for _, item := range items {
		publicationAccess(item)
	}
	return &CreationPublicationPage{Items: items, Total: total, Page: filter.Page, PageSize: filter.PageSize, Pages: (total + int64(filter.PageSize) - 1) / int64(filter.PageSize)}, nil
}

func (s *CreationPublicationService) Open(ctx context.Context, id int64) (*CreationPublication, []byte, error) {
	p, err := s.repo.GetPublic(ctx, id)
	if err != nil {
		return nil, nil, err
	}
	_, store, err := s.resolve(ctx, p.StorageProfileID)
	if err != nil {
		return nil, nil, err
	}
	body, err := store.Get(ctx, p.StorageKey)
	if err != nil {
		return nil, nil, err
	}
	defer func() { _ = body.Close() }()
	data, err := io.ReadAll(io.LimitReader(body, MaxCreationPublicationBytes+1))
	if err != nil {
		return nil, nil, err
	}
	if int64(len(data)) > MaxCreationPublicationBytes {
		return nil, nil, ErrMediaTooLarge
	}
	return publicationAccess(p), data, nil
}

func (s *CreationPublicationService) Delete(ctx context.Context, userID, id int64) error {
	// Repeated owner DELETEs also retry object cleanup, while the tombstone keeps
	// every public read denied even if object storage is temporarily unavailable.
	p, err := s.repo.Withdraw(ctx, userID, id)
	if err != nil {
		return err
	}
	_, store, err := s.resolve(ctx, p.StorageProfileID)
	if err != nil {
		return err
	}
	return cleanupPublicationObject(ctx, store, p.StorageKey)
}
