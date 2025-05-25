package services

import (
	"database/sql"
	"time"

	"github.com/stripe/stripe-go/v75"
)

type StripeService struct {
	DB            *sql.DB
	WebhookSecret string
}

func NewStripeService(secretKey string) *StripeService {
	stripe.Key = secretKey
	return &StripeService{
		WebhookSecret: secretKey,
	}
}

func (ss *StripeService) HandleSubscriptionUpdate(subscription stripe.Subscription) error {
	// Find user by Stripe customer ID
	var userID string
	err := ss.DB.QueryRow(`
		SELECT user_id FROM subscriptions 
		WHERE stripe_customer_id = $1
	`, subscription.Customer.ID).Scan(&userID)

	if err == sql.ErrNoRows {
		// Create new subscription record
		// You'd need to get user_id from somewhere (maybe from subscription metadata)
		return nil
	}

	if err != nil {
		return err
	}

	// Update subscription
	_, err = ss.DB.Exec(`
		INSERT INTO subscriptions (user_id, stripe_subscription_id, stripe_customer_id, status, current_period_start, current_period_end)
		VALUES ($1, $2, $3, $4, $5, $6)
		ON CONFLICT (stripe_subscription_id)
		DO UPDATE SET 
			status = $4,
			current_period_start = $5,
			current_period_end = $6,
			updated_at = NOW()
	`, userID, subscription.ID, subscription.Customer.ID, string(subscription.Status),
		time.Unix(subscription.CurrentPeriodStart, 0),
		time.Unix(subscription.CurrentPeriodEnd, 0))

	if err != nil {
		return err
	}

	// Update user pro status
	isActive := subscription.Status == stripe.SubscriptionStatusActive
	_, err = ss.DB.Exec(`
		UPDATE users SET is_pro = $1 WHERE id = $2
	`, isActive, userID)

	return err
}

func (ss *StripeService) HandleSubscriptionCancellation(subscription stripe.Subscription) error {
	// Update subscription status
	_, err := ss.DB.Exec(`
		UPDATE subscriptions 
		SET status = $1, updated_at = NOW()
		WHERE stripe_subscription_id = $2
	`, string(subscription.Status), subscription.ID)

	if err != nil {
		return err
	}

	// Update user pro status
	_, err = ss.DB.Exec(`
		UPDATE users 
		SET is_pro = FALSE 
		WHERE id = (
			SELECT user_id FROM subscriptions 
			WHERE stripe_subscription_id = $1
		)
	`, subscription.ID)

	return err
}
