package service

import (
	"context"
	"fmt"
	"log/slog"
	"net/mail"
	"strconv"
	"strings"
	"time"

	dbent "github.com/Wei-Shaw/sub2api/ent"
	"github.com/Wei-Shaw/sub2api/ent/invoice"
	"github.com/Wei-Shaw/sub2api/ent/invoiceorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentorder"
	"github.com/Wei-Shaw/sub2api/ent/paymentproviderinstance"
	"github.com/Wei-Shaw/sub2api/internal/config"
	infraerrors "github.com/Wei-Shaw/sub2api/internal/pkg/errors"
)

type invoiceIssuedMailer interface {
	Send(ctx context.Context, input NotificationEmailSendInput) error
	PublicBaseURL(ctx context.Context) string
}

type invoiceMediaAccess interface {
	CreateDownloadGrant(ctx context.Context, in CreateDownloadGrantInput) (*MediaDownloadGrant, error)
}

// InvoiceService handles invoice applications, issuance, and refund interlock.
type InvoiceService struct {
	entClient                *dbent.Client
	frontendURL              string
	notificationEmailService invoiceIssuedMailer
	mediaService             invoiceMediaAccess
}

func NewInvoiceService(entClient *dbent.Client, cfg *config.Config) *InvoiceService {
	frontendURL := ""
	if cfg != nil {
		frontendURL = strings.TrimRight(strings.TrimSpace(cfg.Server.FrontendURL), "/")
	}
	return &InvoiceService{entClient: entClient, frontendURL: frontendURL}
}

func (s *InvoiceService) SetNotificationEmailService(notificationEmailService *NotificationEmailService) {
	s.notificationEmailService = notificationEmailService
}

func (s *InvoiceService) SetMediaService(mediaService *MediaService) {
	s.mediaService = mediaService
}

func (s *InvoiceService) Apply(ctx context.Context, in ApplyInvoiceInput) (*InvoiceView, error) {
	orderIDs := uniquePositiveIDs(in.OrderIDs)
	if len(orderIDs) == 0 {
		return nil, ErrInvoiceNoOrders
	}
	if len(orderIDs) > MaxInvoiceOrders {
		return nil, ErrInvoiceTooManyOrders
	}
	title := strings.TrimSpace(in.Title)
	if title == "" {
		return nil, ErrInvoiceTitleRequired
	}
	if err := validateInvoiceFieldLengths(in); err != nil {
		return nil, err
	}
	email := strings.TrimSpace(in.Email)
	if email == "" {
		return nil, ErrInvoiceEmailRequired
	}
	if _, err := mail.ParseAddress(email); err != nil {
		return nil, infraerrors.BadRequest("INVOICE_EMAIL_INVALID", "invoice email is invalid")
	}
	if strings.TrimSpace(in.TaxNumber) == "" {
		return nil, ErrInvoiceTaxNumberRequired
	}

	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin invoice apply: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	orders, err := queryPaymentOrdersForInvoice(ctx, tx.Client(), orderIDs, true)
	if err != nil {
		return nil, fmt.Errorf("load invoice orders: %w", err)
	}
	owned := make([]*dbent.PaymentOrder, 0, len(orders))
	for _, order := range orders {
		if order.UserID == in.UserID {
			owned = append(owned, order)
		}
	}
	if len(owned) != len(orderIDs) {
		return nil, ErrInvoiceOrderNotEligible
	}
	orders = owned

	occupied, err := tx.InvoiceOrder.Query().
		Where(invoiceorder.OrderIDIn(orderIDs...), invoiceorder.IsActiveEQ(true)).
		Count(ctx)
	if err != nil {
		return nil, fmt.Errorf("check invoice occupancy: %w", err)
	}
	if occupied > 0 {
		return nil, ErrInvoiceOrderOccupied
	}

	enabledByInstance, err := providerInvoiceEnabledBatch(ctx, tx.Client(), orders)
	if err != nil {
		return nil, err
	}
	var amount float64
	currency := ""
	userEmail := strings.TrimSpace(in.UserEmail)
	for _, order := range orders {
		if order.Status != OrderStatusCompleted {
			return nil, ErrInvoiceOrderNotEligible
		}
		if !enabledByInstance[providerInstanceID(order)] {
			return nil, ErrInvoiceChannelDisabled
		}
		orderCurrency := PaymentOrderCurrency(order)
		if currency == "" {
			currency = orderCurrency
		} else if currency != orderCurrency {
			return nil, ErrInvoiceMixedCurrency
		}
		amount += order.PayAmount
		if userEmail == "" {
			userEmail = strings.TrimSpace(order.UserEmail)
		}
	}
	if userEmail == "" {
		if user, err := tx.User.Get(ctx, in.UserID); err == nil && user != nil {
			userEmail = strings.TrimSpace(user.Email)
		}
	}

	now := time.Now()
	inv, err := tx.Invoice.Create().
		SetUserID(in.UserID).
		SetUserEmail(userEmail).
		SetStatus(InvoiceStatusApplied).
		SetUnreadByAdmin(true).
		SetInvoiceAmount(amount).
		SetCurrency(currency).
		SetOrderCount(len(orders)).
		SetTitle(title).
		SetTaxNumber(strings.TrimSpace(in.TaxNumber)).
		SetEmail(email).
		SetContactName(strings.TrimSpace(in.ContactName)).
		SetContactPhone(strings.TrimSpace(in.ContactPhone)).
		SetRequestNote(strings.TrimSpace(in.RequestNote)).
		SetAppliedAt(now).
		Save(ctx)
	if err != nil {
		return nil, fmt.Errorf("create invoice: %w", err)
	}

	builders := make([]*dbent.InvoiceOrderCreate, 0, len(orders))
	for _, order := range orders {
		builders = append(builders, tx.InvoiceOrder.Create().
			SetInvoiceID(inv.ID).
			SetOrderID(order.ID).
			SetPayAmountSnapshot(order.PayAmount).
			SetCurrency(PaymentOrderCurrency(order)).
			SetOutTradeNo(order.OutTradeNo).
			SetPaymentType(order.PaymentType).
			SetIsActive(true))
	}
	if err := tx.InvoiceOrder.CreateBulk(builders...).Exec(ctx); err != nil {
		if dbent.IsConstraintError(err) {
			return nil, ErrInvoiceOrderOccupied
		}
		return nil, fmt.Errorf("link invoice orders: %w", err)
	}

	if err := tx.Commit(); err != nil {
		if dbent.IsConstraintError(err) {
			return nil, ErrInvoiceOrderOccupied
		}
		return nil, fmt.Errorf("commit invoice apply: %w", err)
	}
	committed = true
	return s.Get(ctx, inv.ID, in.UserID, false)
}

func (s *InvoiceService) Get(ctx context.Context, invoiceID, actorUserID int64, isAdmin bool) (*InvoiceView, error) {
	inv, err := s.entClient.Invoice.Get(ctx, invoiceID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrInvoiceNotFound
		}
		return nil, err
	}
	if !isAdmin && inv.UserID != actorUserID {
		return nil, ErrInvoiceForbidden
	}
	return s.viewWithOrders(ctx, inv, true)
}

func (s *InvoiceService) GetAndMarkRead(ctx context.Context, invoiceID, actorUserID int64, isAdmin bool) (*InvoiceView, error) {
	view, err := s.Get(ctx, invoiceID, actorUserID, isAdmin)
	if err != nil {
		return nil, err
	}
	if isAdmin && view.UnreadByAdmin {
		_, _ = s.entClient.Invoice.UpdateOneID(view.ID).SetUnreadByAdmin(false).Save(ctx)
		view.UnreadByAdmin = false
	}
	return view, nil
}

func (s *InvoiceService) List(ctx context.Context, params InvoiceListParams) ([]InvoiceView, int, error) {
	page, pageSize := normalizeInvoicePage(params.Page, params.PageSize)
	q := s.entClient.Invoice.Query()
	if params.UserID > 0 {
		q = q.Where(invoice.UserIDEQ(params.UserID))
	}
	if status := strings.TrimSpace(params.Status); status != "" {
		q = q.Where(invoice.StatusIn(invoiceStatusVariants(status)...))
	}
	if kw := strings.TrimSpace(params.Keyword); kw != "" {
		q = q.Where(invoice.Or(
			invoice.TitleContainsFold(kw),
			invoice.UserEmailContainsFold(kw),
			invoice.TaxNumberContainsFold(kw),
			invoice.EmailContainsFold(kw),
		))
	}
	total, err := q.Clone().Count(ctx)
	if err != nil {
		return nil, 0, err
	}
	rows, err := q.Order(dbent.Desc(invoice.FieldID)).
		Offset((page - 1) * pageSize).
		Limit(pageSize).
		All(ctx)
	if err != nil {
		return nil, 0, err
	}
	out := make([]InvoiceView, 0, len(rows))
	for _, row := range rows {
		out = append(out, invoiceToView(row, nil, invoiceStatusIs(row.Status, InvoiceStatusIssued)))
	}
	return out, total, nil
}

func (s *InvoiceService) CountUnreadApplied(ctx context.Context) (int, error) {
	return s.entClient.Invoice.Query().
		Where(invoice.StatusIn(invoiceStatusVariants(InvoiceStatusApplied)...), invoice.UnreadByAdminEQ(true)).
		Count(ctx)
}

func (s *InvoiceService) Cancel(ctx context.Context, invoiceID, actorUserID int64, isAdmin bool) (*InvoiceView, error) {
	inv, err := s.entClient.Invoice.Get(ctx, invoiceID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrInvoiceNotFound
		}
		return nil, err
	}
	if !isAdmin && inv.UserID != actorUserID {
		return nil, ErrInvoiceForbidden
	}
	if !invoiceStatusIs(inv.Status, InvoiceStatusApplied) {
		return nil, ErrInvoiceInvalidStatus
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin invoice cancel: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	now := time.Now()
	updated, err := tx.Invoice.Update().
		Where(invoice.IDEQ(invoiceID), invoice.StatusIn(invoiceStatusVariants(InvoiceStatusApplied)...)).
		SetStatus(InvoiceStatusCancelled).
		SetCancelledAt(now).
		SetUnreadByAdmin(false).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	if updated == 0 {
		return nil, ErrInvoiceInvalidStatus
	}
	if _, err := tx.InvoiceOrder.Update().
		Where(invoiceorder.InvoiceIDEQ(invoiceID), invoiceorder.IsActiveEQ(true)).
		SetIsActive(false).
		Save(ctx); err != nil {
		return nil, fmt.Errorf("release invoice orders: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit invoice cancel: %w", err)
	}
	committed = true
	return s.Get(ctx, invoiceID, actorUserID, isAdmin)
}

func (s *InvoiceService) Issue(ctx context.Context, invoiceID, mediaID int64) (*InvoiceView, error) {
	if mediaID <= 0 {
		return nil, ErrInvoiceFileRequired
	}
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return nil, fmt.Errorf("begin invoice issue: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()

	inv, err := queryInvoiceForUpdate(ctx, tx.Client(), invoiceID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrInvoiceNotFound
		}
		return nil, err
	}
	if !invoiceStatusIs(inv.Status, InvoiceStatusApplied) {
		return nil, ErrInvoiceInvalidStatus
	}
	if err := s.ensureLinkedOrdersStillCompletable(ctx, tx.Client(), invoiceID); err != nil {
		return nil, err
	}
	asset, err := tx.MediaAsset.Get(ctx, mediaID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return nil, ErrInvoiceFileInvalid
		}
		return nil, err
	}
	if err := validateInvoiceMedia(asset, inv); err != nil {
		return nil, err
	}
	if strings.TrimSpace(asset.BizID) == "" {
		_, err = tx.MediaAsset.UpdateOneID(asset.ID).
			SetBizID(strconv.FormatInt(inv.ID, 10)).
			Save(ctx)
		if err != nil {
			return nil, fmt.Errorf("attach invoice media: %w", err)
		}
	}

	now := time.Now()
	updated, err := tx.Invoice.Update().
		Where(invoice.IDEQ(invoiceID), invoice.StatusIn(invoiceStatusVariants(InvoiceStatusApplied)...)).
		SetStatus(InvoiceStatusIssued).
		SetIssuedAt(now).
		SetFileMediaID(mediaID).
		SetFileName(asset.Filename).
		SetFileMimeType(asset.Mime).
		SetFileSizeBytes(asset.Size).
		SetUnreadByAdmin(false).
		Save(ctx)
	if err != nil {
		return nil, err
	}
	if updated == 0 {
		return nil, ErrInvoiceInvalidStatus
	}
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit invoice issue: %w", err)
	}
	committed = true

	view, err := s.Get(ctx, invoiceID, inv.UserID, true)
	if err != nil {
		return nil, err
	}
	s.sendIssuedEmail(ctx, view)
	return view, nil
}

func (s *InvoiceService) ensureLinkedOrdersStillCompletable(ctx context.Context, client *dbent.Client, invoiceID int64) error {
	links, err := client.InvoiceOrder.Query().
		Where(invoiceorder.InvoiceIDEQ(invoiceID), invoiceorder.IsActiveEQ(true)).
		All(ctx)
	if err != nil {
		return err
	}
	if len(links) == 0 {
		return ErrInvoiceOrderNotEligible
	}
	orderIDs := make([]int64, 0, len(links))
	for _, link := range links {
		orderIDs = append(orderIDs, link.OrderID)
	}
	orders, err := queryPaymentOrdersForInvoice(ctx, client, orderIDs, true)
	if err != nil {
		return err
	}
	if len(orders) != len(orderIDs) {
		return ErrInvoiceOrderNotEligible
	}
	for _, order := range orders {
		if order.Status != OrderStatusCompleted {
			return ErrInvoiceOrderNotEligible
		}
	}
	return nil
}

func (s *InvoiceService) ResendIssuedEmail(ctx context.Context, invoiceID int64) error {
	inv, err := s.entClient.Invoice.Get(ctx, invoiceID)
	if err != nil {
		if dbent.IsNotFound(err) {
			return ErrInvoiceNotFound
		}
		return err
	}
	if !invoiceStatusIs(inv.Status, InvoiceStatusIssued) {
		return ErrInvoiceInvalidStatus
	}
	view, err := s.viewWithOrders(ctx, inv, true)
	if err != nil {
		return err
	}
	return s.sendIssuedEmailErr(ctx, view, "resend:"+strconv.FormatInt(time.Now().UnixNano(), 10))
}

func (s *InvoiceService) HasActiveIssuedInvoiceForOrder(ctx context.Context, orderID int64) (bool, error) {
	return s.entClient.InvoiceOrder.Query().
		Where(
			invoiceorder.OrderIDEQ(orderID),
			invoiceorder.IsActiveEQ(true),
			invoiceorder.HasInvoiceWith(invoice.StatusIn(invoiceStatusVariants(InvoiceStatusIssued)...)),
		).
		Exist(ctx)
}

func (s *InvoiceService) CancelAppliedInvoicesForOrder(ctx context.Context, orderID int64) error {
	tx, err := s.entClient.Tx(ctx)
	if err != nil {
		return fmt.Errorf("begin cancel applied invoices: %w", err)
	}
	committed := false
	defer func() {
		if !committed {
			_ = tx.Rollback()
		}
	}()
	if err := s.cancelActiveInvoices(ctx, tx.Client(), orderID, false); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	committed = true
	return nil
}

func (s *InvoiceService) CancelAppliedInvoicesForOrderWithClient(ctx context.Context, client *dbent.Client, orderID int64) error {
	if client == nil {
		client = s.entClient
	}
	return s.cancelActiveInvoices(ctx, client, orderID, false)
}

func (s *InvoiceService) CancelActiveInvoicesForRefund(ctx context.Context, client *dbent.Client, orderID int64, force bool) error {
	if client == nil {
		client = s.entClient
	}
	return s.cancelActiveInvoices(ctx, client, orderID, force)
}

func (s *InvoiceService) cancelActiveInvoices(ctx context.Context, client *dbent.Client, orderID int64, includeIssued bool) error {
	// ISSUED invoices must be credit-noted (红冲) by finance; never auto-cancel them.
	_ = includeIssued
	statuses := invoiceStatusVariants(InvoiceStatusApplied)
	links, err := client.InvoiceOrder.Query().
		Where(
			invoiceorder.OrderIDEQ(orderID),
			invoiceorder.IsActiveEQ(true),
			invoiceorder.HasInvoiceWith(invoice.StatusIn(statuses...)),
		).
		All(ctx)
	if err != nil {
		return err
	}
	if len(links) == 0 {
		return nil
	}
	invoiceIDs := make([]int64, 0, len(links))
	seen := map[int64]struct{}{}
	for _, link := range links {
		if _, ok := seen[link.InvoiceID]; ok {
			continue
		}
		seen[link.InvoiceID] = struct{}{}
		invoiceIDs = append(invoiceIDs, link.InvoiceID)
	}
	now := time.Now()
	if _, err := client.Invoice.Update().
		Where(invoice.IDIn(invoiceIDs...), invoice.StatusIn(statuses...)).
		SetStatus(InvoiceStatusCancelled).
		SetCancelledAt(now).
		SetUnreadByAdmin(false).
		Save(ctx); err != nil {
		return err
	}
	_, err = client.InvoiceOrder.Update().
		Where(invoiceorder.InvoiceIDIn(invoiceIDs...), invoiceorder.IsActiveEQ(true)).
		SetIsActive(false).
		Save(ctx)
	return err
}

func (s *InvoiceService) InvoiceDetailURL(ctx context.Context, invoiceID int64) string {
	base := strings.TrimRight(strings.TrimSpace(s.frontendURL), "/")
	if base == "" && s.notificationEmailService != nil {
		base = strings.TrimRight(strings.TrimSpace(s.notificationEmailService.PublicBaseURL(ctx)), "/")
	}
	if base == "" || !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		return ""
	}
	return fmt.Sprintf("%s/invoices/%d", base, invoiceID)
}

func (s *InvoiceService) viewWithOrders(ctx context.Context, inv *dbent.Invoice, includeFile bool) (*InvoiceView, error) {
	links, err := s.entClient.InvoiceOrder.Query().
		Where(invoiceorder.InvoiceIDEQ(inv.ID)).
		Order(invoiceorder.ByID()).
		All(ctx)
	if err != nil {
		return nil, err
	}
	view := invoiceToView(inv, links, includeFile && invoiceStatusIs(inv.Status, InvoiceStatusIssued))
	return &view, nil
}

func (s *InvoiceService) sendIssuedEmail(ctx context.Context, view *InvoiceView) {
	if err := s.sendIssuedEmailErr(ctx, view, "auto"); err != nil {
		slog.Warn("invoice issued email failed", "invoice_id", view.ID, "err", err.Error())
	}
}

func (s *InvoiceService) sendIssuedEmailErr(ctx context.Context, view *InvoiceView, reminderKey string) error {
	if s.notificationEmailService == nil || view == nil {
		return nil
	}
	downloadURL, err := s.issuedEmailDownloadURL(ctx, view)
	if err != nil {
		return err
	}
	detailURL := s.InvoiceDetailURL(ctx, view.ID)
	amountDisplay := fmt.Sprintf("%.2f", view.InvoiceAmount)
	if strings.TrimSpace(view.Currency) != "" {
		amountDisplay = amountDisplay + " " + strings.TrimSpace(view.Currency)
	}
	return s.notificationEmailService.Send(ctx, NotificationEmailSendInput{
		Event:          NotificationEmailEventInvoiceIssued,
		RecipientEmail: view.Email,
		RecipientName:  firstNonEmpty(view.ContactName, view.UserEmail, view.Email),
		UserID:         view.UserID,
		SourceType:     "invoice",
		SourceID:       strconv.FormatInt(view.ID, 10),
		ReminderKey:    reminderKey,
		Variables: map[string]string{
			"invoice_id":             strconv.FormatInt(view.ID, 10),
			"invoice_title":          view.Title,
			"tax_number":             view.TaxNumber,
			"invoice_amount":         fmt.Sprintf("%.2f", view.InvoiceAmount),
			"invoice_amount_display": amountDisplay,
			"order_count":            strconv.Itoa(view.OrderCount),
			"invoice_file_name":      view.FileName,
			"invoice_download_url":   downloadURL,
			"detail_url":             detailURL,
		},
		RawHTMLVariables: map[string]string{
			"order_list_html": renderInvoiceOrderListHTML(view.Orders),
		},
	})
}

func (s *InvoiceService) issuedEmailDownloadURL(ctx context.Context, view *InvoiceView) (string, error) {
	if s.mediaService == nil || view == nil || view.FileMediaID == nil || *view.FileMediaID <= 0 {
		return "", ErrInvoiceDownloadURLUnavailable
	}
	grant, err := s.mediaService.CreateDownloadGrant(ctx, CreateDownloadGrantInput{
		AssetID:        *view.FileMediaID,
		ActorUserID:    view.UserID,
		VerifiedAccess: true,
		TTLMinutes:     InvoiceEmailDownloadTTLMinutes,
		MaxTTLMinutes:  InvoiceEmailDownloadTTLMinutes,
	})
	if err != nil {
		return "", err
	}
	if grant == nil {
		return "", ErrInvoiceDownloadURLUnavailable
	}
	url := AbsoluteMediaURL(s.apiBaseURL(ctx), grant.URL)
	if url == "" {
		return "", ErrInvoiceDownloadURLUnavailable
	}
	return url, nil
}

func (s *InvoiceService) apiBaseURL(ctx context.Context) string {
	if s.notificationEmailService != nil {
		if base := strings.TrimRight(strings.TrimSpace(s.notificationEmailService.PublicBaseURL(ctx)), "/"); base != "" {
			return base
		}
	}
	return strings.TrimRight(strings.TrimSpace(s.frontendURL), "/")
}

func renderInvoiceOrderListHTML(orders []InvoiceOrderItem) string {
	if len(orders) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteString(`<table style="width:100%;border-collapse:collapse">`)
	for _, order := range orders {
		b.WriteString(`<tr><td style="padding:4px 0;font-family:monospace">#`)
		b.WriteString(strconv.FormatInt(order.OrderID, 10))
		if order.OutTradeNo != "" {
			b.WriteString(" ")
			b.WriteString(order.OutTradeNo)
		}
		b.WriteString(`</td><td style="padding:4px 0;text-align:right">`)
		b.WriteString(fmt.Sprintf("%.2f", order.PayAmountSnapshot))
		if order.Currency != "" {
			b.WriteString(" ")
			b.WriteString(order.Currency)
		}
		b.WriteString(`</td></tr>`)
	}
	b.WriteString(`</table>`)
	return b.String()
}

func validateInvoiceMedia(asset *dbent.MediaAsset, inv *dbent.Invoice) error {
	if asset.Status != MediaStatusReady {
		return ErrInvoiceFileInvalid
	}
	if asset.BizType != MediaBizInvoice {
		return ErrInvoiceFileInvalid
	}
	if asset.Visibility != MediaVisibilityPrivate {
		return ErrInvoiceFileInvalid
	}
	if asset.OwnerUserID != inv.UserID {
		return ErrInvoiceFileInvalid
	}
	bizID := strings.TrimSpace(asset.BizID)
	if bizID != "" && bizID != strconv.FormatInt(inv.ID, 10) {
		return ErrInvoiceFileInvalid
	}
	return nil
}

func validateInvoiceFieldLengths(in ApplyInvoiceInput) error {
	if len([]rune(strings.TrimSpace(in.Title))) > 200 {
		return ErrInvoiceTitleTooLong
	}
	if len([]rune(strings.TrimSpace(in.TaxNumber))) > 64 ||
		len([]rune(strings.TrimSpace(in.Email))) > 255 ||
		len([]rune(strings.TrimSpace(in.ContactName))) > 100 ||
		len([]rune(strings.TrimSpace(in.ContactPhone))) > 40 {
		return ErrInvoiceFieldTooLong
	}
	return nil
}

func providerInstanceID(order *dbent.PaymentOrder) int64 {
	if order == nil || order.ProviderInstanceID == nil {
		return 0
	}
	id, err := strconv.ParseInt(strings.TrimSpace(*order.ProviderInstanceID), 10, 64)
	if err != nil {
		return 0
	}
	return id
}

func providerInvoiceEnabledBatch(ctx context.Context, client *dbent.Client, orders []*dbent.PaymentOrder) (map[int64]bool, error) {
	out := make(map[int64]bool, len(orders))
	ids := make([]int64, 0, len(orders))
	seen := map[int64]struct{}{}
	for _, order := range orders {
		id := providerInstanceID(order)
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		ids = append(ids, id)
	}
	if len(ids) == 0 {
		return out, nil
	}
	instances, err := client.PaymentProviderInstance.Query().
		Where(paymentproviderinstance.IDIn(ids...)).
		All(ctx)
	if err != nil {
		return nil, err
	}
	for _, inst := range instances {
		out[inst.ID] = inst.InvoiceEnabled
	}
	return out, nil
}

func invoiceToView(inv *dbent.Invoice, links []*dbent.InvoiceOrder, includeFile bool) InvoiceView {
	view := InvoiceView{
		ID:            inv.ID,
		UserID:        inv.UserID,
		UserEmail:     inv.UserEmail,
		Status:        normalizeInvoiceStatus(inv.Status),
		UnreadByAdmin: inv.UnreadByAdmin,
		InvoiceAmount: inv.InvoiceAmount,
		Currency:      inv.Currency,
		OrderCount:    inv.OrderCount,
		Title:         inv.Title,
		TaxNumber:     inv.TaxNumber,
		Email:         inv.Email,
		ContactName:   inv.ContactName,
		ContactPhone:  inv.ContactPhone,
		RequestNote:   inv.RequestNote,
		AppliedAt:     inv.AppliedAt,
		CancelledAt:   inv.CancelledAt,
		IssuedAt:      inv.IssuedAt,
		CreatedAt:     inv.CreatedAt,
		UpdatedAt:     inv.UpdatedAt,
	}
	if includeFile && inv.FileMediaID != nil && *inv.FileMediaID > 0 {
		id := *inv.FileMediaID
		view.FileMediaID = &id
		view.FileName = inv.FileName
		view.FileMimeType = inv.FileMimeType
		view.FileSizeBytes = inv.FileSizeBytes
		view.HasFile = true
	}
	if len(links) > 0 {
		view.Orders = make([]InvoiceOrderItem, 0, len(links))
		for _, link := range links {
			view.Orders = append(view.Orders, InvoiceOrderItem{
				OrderID:           link.OrderID,
				PayAmountSnapshot: link.PayAmountSnapshot,
				Currency:          link.Currency,
				OutTradeNo:        link.OutTradeNo,
				PaymentType:       link.PaymentType,
				IsActive:          link.IsActive,
				CreatedAt:         link.CreatedAt,
			})
		}
	}
	return view
}

func normalizeInvoiceStatus(status string) string {
	return strings.ToUpper(strings.TrimSpace(status))
}

func invoiceStatusIs(status, expected string) bool {
	return normalizeInvoiceStatus(status) == normalizeInvoiceStatus(expected)
}

func invoiceStatusVariants(status string) []string {
	canonical := normalizeInvoiceStatus(status)
	if canonical == "" {
		return nil
	}
	return []string{canonical, strings.ToLower(canonical)}
}

func queryInvoiceForUpdate(ctx context.Context, client *dbent.Client, invoiceID int64) (*dbent.Invoice, error) {
	inv, err := client.Invoice.Query().Where(invoice.IDEQ(invoiceID)).ForUpdate().Only(ctx)
	if err != nil && isSQLiteForUpdateUnsupported(err) {
		return client.Invoice.Get(ctx, invoiceID)
	}
	return inv, err
}

func queryPaymentOrdersForInvoice(ctx context.Context, client *dbent.Client, orderIDs []int64, lock bool) ([]*dbent.PaymentOrder, error) {
	q := client.PaymentOrder.Query().Where(paymentorder.IDIn(orderIDs...))
	if lock {
		q = q.ForUpdate()
	}
	orders, err := q.All(ctx)
	if err != nil && lock && isSQLiteForUpdateUnsupported(err) {
		return client.PaymentOrder.Query().Where(paymentorder.IDIn(orderIDs...)).All(ctx)
	}
	return orders, err
}

func isSQLiteForUpdateUnsupported(err error) bool {
	return err != nil && strings.Contains(err.Error(), "FOR UPDATE/SHARE not supported")
}

func uniquePositiveIDs(ids []int64) []int64 {
	seen := make(map[int64]struct{}, len(ids))
	out := make([]int64, 0, len(ids))
	for _, id := range ids {
		if id <= 0 {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		out = append(out, id)
	}
	return out
}

func normalizeInvoicePage(page, pageSize int) (int, int) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	return page, pageSize
}
