package billing

import (
	"context"
	"encoding/json"
	"strconv"
	"time"

	"github.com/stripe/stripe-go/v85"
	"github.com/stripe/stripe-go/v85/webhook"
)

// StripeProvider membungkus klien resmi stripe-go. baseURL kosong = api.stripe.com; di dev diarahkan ke emulator.
type StripeProvider struct {
	sc            *stripe.Client
	webhookSecret string
	priceID       string
	amount        int64 // satuan utama
	currency      string
}

func NewStripeProvider(apiKey, baseURL, webhookSecret, priceID string, amount int64, currency string) *StripeProvider {
	var opts []stripe.ClientOption
	if baseURL != "" {
		opts = append(opts, stripe.WithBackends(stripe.NewBackendsWithConfig(&stripe.BackendConfig{URL: stripe.String(baseURL)})))
	}
	return &StripeProvider{
		sc:            stripe.NewClient(apiKey, opts...),
		webhookSecret: webhookSecret,
		priceID:       priceID,
		amount:        amount,
		currency:      currency,
	}
}

// CreateCheckout membuka sesi pembayaran sekali bayar untuk satu bulan langganan.
func (p *StripeProvider) CreateCheckout(ctx context.Context, in CheckoutInput) (Checkout, error) {
	user := strconv.FormatInt(in.UserID, 10)
	params := &stripe.CheckoutSessionCreateParams{
		Mode:              stripe.String(string(stripe.CheckoutSessionModePayment)),
		SuccessURL:        stripe.String(in.SuccessURL),
		CancelURL:         stripe.String(in.CancelURL),
		CustomerEmail:     stripe.String(in.CustomerEmail),
		ClientReferenceID: stripe.String(user),
		Metadata:          map[string]string{"user_id": user, "purpose": "qms_subscription_1_month"},
	}
	item := &stripe.CheckoutSessionCreateLineItemParams{Quantity: stripe.Int64(1)}
	if p.priceID != "" {
		item.Price = stripe.String(p.priceID)
	} else {
		item.PriceData = &stripe.CheckoutSessionCreateLineItemPriceDataParams{
			Currency:   stripe.String(p.currency),
			UnitAmount: stripe.Int64(p.amount * MinorFactor(p.currency)),
			ProductData: &stripe.CheckoutSessionCreateLineItemPriceDataProductDataParams{
				Name: stripe.String("Langganan QMS 1 bulan"),
			},
		}
	}
	params.LineItems = []*stripe.CheckoutSessionCreateLineItemParams{item}

	sess, err := p.sc.V1CheckoutSessions.Create(ctx, params)
	if err != nil {
		return Checkout{}, err
	}
	return Checkout{URL: sess.URL, ExpiresAt: time.Unix(sess.ExpiresAt, 0).UTC()}, nil
}

// ParseEvent memverifikasi tanda tangan dan mengurai bagian yang dibutuhkan. Selisih versi API diabaikan
// karena kita hanya membaca beberapa field stabil.
func (p *StripeProvider) ParseEvent(payload []byte, sigHeader string) (Event, error) {
	ev, err := webhook.ConstructEventWithOptions(payload, sigHeader, p.webhookSecret,
		webhook.ConstructEventOptions{IgnoreAPIVersionMismatch: true})
	if err != nil {
		return Event{}, err
	}
	out := Event{ID: ev.ID, Type: string(ev.Type)}

	var obj struct {
		ID                string `json:"id"`
		ClientReferenceID string `json:"client_reference_id"`
		PaymentStatus     string `json:"payment_status"`
		AmountTotal       int64  `json:"amount_total"`
		Currency          string `json:"currency"`
	}
	if ev.Data != nil && len(ev.Data.Raw) > 0 {
		_ = json.Unmarshal(ev.Data.Raw, &obj)
	}
	user, _ := strconv.ParseInt(obj.ClientReferenceID, 10, 64)
	out.Session = Session{
		ID:            obj.ID,
		UserID:        user,
		PaymentStatus: obj.PaymentStatus,
		AmountTotal:   obj.AmountTotal,
		Currency:      obj.Currency,
	}
	return out, nil
}
