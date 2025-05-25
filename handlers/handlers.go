package handlers

import (
	"email-drip-be/services"
	"encoding/json"
	"io"
	"strconv"

	"github.com/gin-gonic/gin"
	"github.com/stripe/stripe-go/v75"
	"github.com/stripe/stripe-go/v75/webhook"
)

type Handlers struct {
	AI     *services.AIService
	Email  *services.EmailService
	Stripe *services.StripeService
}

type RewriteRequest struct {
	Email  string `json:"email" binding:"required"`
	Tone   string `json:"tone" binding:"required"`
	Roast  bool   `json:"roast"`
	UserID string `json:"user_id" binding:"required"`
}

type RewriteResponse struct {
	Rewritten string `json:"rewritten"`
	Roast     string `json:"roast,omitempty"`
}

func (h *Handlers) RewriteEmail(c *gin.Context) {
	var req RewriteRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": err.Error()})
		return
	}

	// Check if user can make requests (usage limit or pro status)
	canUse, err := h.Email.CanUserMakeRequest(req.UserID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to check user limits"})
		return
	}

	if !canUse {
		c.JSON(429, gin.H{"error": "Daily limit reached. Upgrade to Pro for unlimited emails."})
		return
	}

	// Generate AI rewrite
	rewritten, err := h.AI.RewriteEmail(req.Email, req.Tone)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to rewrite email"})
		return
	}

	response := RewriteResponse{
		Rewritten: rewritten,
	}

	// Generate roast if requested
	if req.Roast {
		roast, err := h.AI.RoastEmail(req.Email)
		if err == nil {
			response.Roast = roast
		}
	}

	// Save to database and increment usage
	emailRecord := services.EmailRecord{
		UserID:    req.UserID,
		Original:  req.Email,
		Rewritten: rewritten,
		Roast:     response.Roast,
		Tone:      req.Tone,
		RoastMode: req.Roast,
	}

	if err := h.Email.SaveEmail(emailRecord); err != nil {
		c.JSON(500, gin.H{"error": "Failed to save email"})
		return
	}

	if err := h.Email.IncrementUsage(req.UserID); err != nil {
		c.JSON(500, gin.H{"error": "Failed to update usage"})
		return
	}

	c.JSON(200, response)
}

func (h *Handlers) GetUsage(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(400, gin.H{"error": "User ID required"})
		return
	}

	usage, err := h.Email.GetUserUsage(userID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get usage"})
		return
	}

	isPro, err := h.Email.IsUserPro(userID)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to check pro status"})
		return
	}

	c.JSON(200, gin.H{
		"usage":  usage,
		"is_pro": isPro,
		"limit":  5,
	})
}

func (h *Handlers) GetUserEmails(c *gin.Context) {
	userID := c.Param("user_id")
	if userID == "" {
		c.JSON(400, gin.H{"error": "User ID required"})
		return
	}

	limitStr := c.DefaultQuery("limit", "10")
	limit, err := strconv.Atoi(limitStr)
	if err != nil {
		limit = 10
	}

	emails, err := h.Email.GetUserEmails(userID, limit)
	if err != nil {
		c.JSON(500, gin.H{"error": "Failed to get emails"})
		return
	}

	c.JSON(200, gin.H{"emails": emails})
}

func (h *Handlers) StripeWebhook(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid payload"})
		return
	}

	event, err := webhook.ConstructEvent(payload, c.GetHeader("Stripe-Signature"), h.Stripe.WebhookSecret)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid signature"})
		return
	}

	switch event.Type {
	case "customer.subscription.created", "customer.subscription.updated":
		var subscription stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
			c.JSON(400, gin.H{"error": "Invalid subscription data"})
			return
		}

		if err := h.Stripe.HandleSubscriptionUpdate(subscription); err != nil {
			c.JSON(500, gin.H{"error": "Failed to update subscription"})
			return
		}

	case "customer.subscription.deleted":
		var subscription stripe.Subscription
		if err := json.Unmarshal(event.Data.Raw, &subscription); err != nil {
			c.JSON(400, gin.H{"error": "Invalid subscription data"})
			return
		}

		if err := h.Stripe.HandleSubscriptionCancellation(subscription); err != nil {
			c.JSON(500, gin.H{"error": "Failed to cancel subscription"})
			return
		}
	}

	c.JSON(200, gin.H{"received": true})
}
