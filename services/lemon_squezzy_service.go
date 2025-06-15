package services

import (
	"database/sql"
	"time"
)

type LemonSqueezyService struct {
	DB            *sql.DB
	WebhookSecret string
	APIKey        string
}

type LemonSqueezySubscription struct {
	ID            string                 `json:"id"`
	Type          string                 `json:"type"`
	Attributes    SubscriptionAttributes `json:"attributes"`
	Relationships SubscriptionRelations  `json:"relationships"`
}

type SubscriptionAttributes struct {
	StoreID              int           `json:"store_id"`
	CustomerID           int           `json:"customer_id"`
	OrderID              int           `json:"order_id"`
	OrderItemID          int           `json:"order_item_id"`
	ProductID            int           `json:"product_id"`
	VariantID            int           `json:"variant_id"`
	ProductName          string        `json:"product_name"`
	VariantName          string        `json:"variant_name"`
	UserName             string        `json:"user_name"`
	UserEmail            string        `json:"user_email"`
	Status               string        `json:"status"`
	StatusFormatted      string        `json:"status_formatted"`
	CardBrand            string        `json:"card_brand"`
	CardLastFour         string        `json:"card_last_four"`
	PausedAt             *time.Time    `json:"paused_at"`
	SubscriptionItemID   int           `json:"subscription_item_id"`
	URLs                 URLs          `json:"urls"`
	RenewsAt             time.Time     `json:"renews_at"`
	EndsAt               *time.Time    `json:"ends_at"`
	TrialEndsAt          *time.Time    `json:"trial_ends_at"`
	Price                string        `json:"price"`
	IsUsageBased         bool          `json:"is_usage_based"`
	IsPaused             bool          `json:"is_paused"`
	SubscriptionInvoices []interface{} `json:"subscription_invoices"`
	CreatedAt            time.Time     `json:"created_at"`
	UpdatedAt            time.Time     `json:"updated_at"`
	TestMode             bool          `json:"test_mode"`
}

type SubscriptionRelations struct {
	Store                Store                `json:"store"`
	Customer             Customer             `json:"customer"`
	Order                Order                `json:"order"`
	OrderItem            OrderItem            `json:"order_item"`
	Product              Product              `json:"product"`
	Variant              Variant              `json:"variant"`
	SubscriptionInvoices SubscriptionInvoices `json:"subscription-invoices"`
}

type Store struct {
	Links Links `json:"links"`
}

type Customer struct {
	Links Links `json:"links"`
}

type Order struct {
	Links Links `json:"links"`
}

type OrderItem struct {
	Links Links `json:"links"`
}

type Product struct {
	Links Links `json:"links"`
}

type Variant struct {
	Links Links `json:"links"`
}

type SubscriptionInvoices struct {
	Links Links `json:"links"`
}

type Links struct {
	Related string `json:"related"`
	Self    string `json:"self"`
}

type URLs struct {
	UpdatePaymentMethod string `json:"update_payment_method"`
}

func NewLemonSqueezyService(apiKey string, webhookSecret string, db *sql.DB) *LemonSqueezyService {
	return &LemonSqueezyService{
		DB:            db,
		WebhookSecret: webhookSecret,
		APIKey:        apiKey,
	}
}

func (ls *LemonSqueezyService) HandleSubscriptionCreated(subscription LemonSqueezySubscription) error {
	// Insert or update subscription
	_, err := ls.DB.Exec(`
		INSERT INTO subscriptions (
			user_id, 
			lemonsqueezy_subscription_id, 
			lemonsqueezy_customer_id, 
			status, 
			current_period_start, 
			current_period_end,
			created_at,
			updated_at
		)
		VALUES (
			(SELECT id FROM users WHERE email = $1), 
			$2, 
			$3, 
			$4, 
			$5, 
			$6,
			NOW(),
			NOW()
		)
		ON CONFLICT (lemonsqueezy_subscription_id)
		DO UPDATE SET 
			status = $4,
			current_period_start = $5,
			current_period_end = $6,
			updated_at = NOW()
	`,
		subscription.Attributes.UserEmail,
		subscription.ID,
		subscription.Attributes.CustomerID,
		subscription.Attributes.Status,
		subscription.Attributes.CreatedAt,
		subscription.Attributes.RenewsAt,
	)

	if err != nil {
		return err
	}

	// Update user pro status
	isActive := subscription.Attributes.Status == "active"
	_, err = ls.DB.Exec(`
		UPDATE users 
		SET is_pro = $1, updated_at = NOW()
		WHERE email = $2
	`, isActive, subscription.Attributes.UserEmail)

	return err
}

func (ls *LemonSqueezyService) HandleSubscriptionUpdated(subscription LemonSqueezySubscription) error {
	// Update subscription
	_, err := ls.DB.Exec(`
		UPDATE subscriptions 
		SET 
			status = $1,
			current_period_start = $2,
			current_period_end = $3,
			updated_at = NOW()
		WHERE lemonsqueezy_subscription_id = $4
	`,
		subscription.Attributes.Status,
		subscription.Attributes.CreatedAt,
		subscription.Attributes.RenewsAt,
		subscription.ID,
	)

	if err != nil {
		return err
	}

	// Update user pro status
	isActive := subscription.Attributes.Status == "active"
	_, err = ls.DB.Exec(`
		UPDATE users 
		SET is_pro = $1, updated_at = NOW()
		WHERE id = (
			SELECT user_id FROM subscriptions 
			WHERE lemonsqueezy_subscription_id = $2
		)
	`, isActive, subscription.ID)

	return err
}

func (ls *LemonSqueezyService) HandleSubscriptionCancelled(subscription LemonSqueezySubscription) error {
	// Update subscription status
	_, err := ls.DB.Exec(`
		UPDATE subscriptions 
		SET 
			status = $1, 
			updated_at = NOW()
		WHERE lemonsqueezy_subscription_id = $2
	`, subscription.Attributes.Status, subscription.ID)

	if err != nil {
		return err
	}

	// Update user pro status to false
	_, err = ls.DB.Exec(`
		UPDATE users 
		SET is_pro = FALSE, updated_at = NOW()
		WHERE id = (
			SELECT user_id FROM subscriptions 
			WHERE lemonsqueezy_subscription_id = $1
		)
	`, subscription.ID)

	return err
}

func (ls *LemonSqueezyService) HandleSubscriptionResumed(subscription LemonSqueezySubscription) error {
	return ls.HandleSubscriptionUpdated(subscription)
}

func (ls *LemonSqueezyService) HandleSubscriptionExpired(subscription LemonSqueezySubscription) error {
	return ls.HandleSubscriptionCancelled(subscription)
}

func (ls *LemonSqueezyService) HandleSubscriptionPaused(subscription LemonSqueezySubscription) error {
	// Update subscription status
	_, err := ls.DB.Exec(`
		UPDATE subscriptions 
		SET 
			status = $1, 
			updated_at = NOW()
		WHERE lemonsqueezy_subscription_id = $2
	`, subscription.Attributes.Status, subscription.ID)

	if err != nil {
		return err
	}

	// Keep pro status active for paused subscriptions
	// You might want to change this behavior based on your needs
	return nil
}

func (ls *LemonSqueezyService) HandleSubscriptionUnpaused(subscription LemonSqueezySubscription) error {
	return ls.HandleSubscriptionUpdated(subscription)
}
