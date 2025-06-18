package handlers

import (
	"crypto/hmac"
	"crypto/sha256"
	"emaildrip-be/services"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"

	"github.com/gin-gonic/gin"
)

type Handlers struct {
	AI           *services.AIService
	Email        *services.EmailService
	LemonSqueezy *services.LemonSqueezyService
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

type LemonSqueezyWebhookPayload struct {
	Meta LemonSqueezyMeta                  `json:"meta"`
	Data services.LemonSqueezySubscription `json:"data"`
}

type LemonSqueezyMeta struct {
	EventName  string                 `json:"event_name"`
	CustomData map[string]interface{} `json:"custom_data"`
}

type CheckoutRequest struct {
	UserID string `json:"user_id" binding:"required"`
	Email  string `json:"email" binding:"required"`
}

type CheckoutResponse struct {
	URL string `json:"url"`
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

// verifyLemonSqueezySignature verifies the webhook signature from LemonSqueezy
func (h *Handlers) verifyLemonSqueezySignature(payload []byte, signature string, secret string) bool {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(payload)
	expectedMAC := hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(signature), []byte(expectedMAC))
}

func (h *Handlers) LemonSqueezyWebhook(c *gin.Context) {
	payload, err := io.ReadAll(c.Request.Body)
	if err != nil {
		c.JSON(400, gin.H{"error": "Invalid payload"})
		return
	}

	// Verify webhook signature
	signature := c.GetHeader("X-Signature")
	if !h.verifyLemonSqueezySignature(payload, signature, h.LemonSqueezy.WebhookSecret) {
		c.JSON(400, gin.H{"error": "Invalid signature"})
		return
	}

	// Parse webhook payload
	var webhookPayload LemonSqueezyWebhookPayload
	if err := json.Unmarshal(payload, &webhookPayload); err != nil {
		c.JSON(400, gin.H{"error": "Invalid JSON payload"})
		return
	}

	// Handle different event types
	switch webhookPayload.Meta.EventName {
	case "subscription_created":
		if err := h.LemonSqueezy.HandleSubscriptionCreated(webhookPayload.Data); err != nil {
			c.JSON(500, gin.H{"error": "Failed to handle subscription creation"})
			return
		}

	case "subscription_updated":
		if err := h.LemonSqueezy.HandleSubscriptionUpdated(webhookPayload.Data); err != nil {
			c.JSON(500, gin.H{"error": "Failed to handle subscription update"})
			return
		}

	case "subscription_cancelled":
		if err := h.LemonSqueezy.HandleSubscriptionCancelled(webhookPayload.Data); err != nil {
			c.JSON(500, gin.H{"error": "Failed to handle subscription cancellation"})
			return
		}

	case "subscription_resumed":
		if err := h.LemonSqueezy.HandleSubscriptionResumed(webhookPayload.Data); err != nil {
			c.JSON(500, gin.H{"error": "Failed to handle subscription resumption"})
			return
		}

	case "subscription_expired":
		if err := h.LemonSqueezy.HandleSubscriptionExpired(webhookPayload.Data); err != nil {
			c.JSON(500, gin.H{"error": "Failed to handle subscription expiration"})
			return
		}

	case "subscription_paused":
		if err := h.LemonSqueezy.HandleSubscriptionPaused(webhookPayload.Data); err != nil {
			c.JSON(500, gin.H{"error": "Failed to handle subscription pause"})
			return
		}

	case "subscription_unpaused":
		if err := h.LemonSqueezy.HandleSubscriptionUnpaused(webhookPayload.Data); err != nil {
			c.JSON(500, gin.H{"error": "Failed to handle subscription unpause"})
			return
		}

	default:
		// Log unknown event type but return success to avoid retries
		c.JSON(200, gin.H{"message": "Unknown event type", "event": webhookPayload.Meta.EventName})
		return
	}

	c.JSON(200, gin.H{"received": true})
}

func (h *Handlers) CreateCheckout(c *gin.Context) {
	var req CheckoutRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(400, gin.H{"error": fmt.Sprintf("Invalid request: %v", err)})
		return
	}

	// Validate required fields
	if req.UserID == "" || req.Email == "" {
		c.JSON(400, gin.H{"error": "user_id and email are required"})
		return
	}

	// Create checkout session with LemonSqueezy
	checkoutURL, err := h.LemonSqueezy.CreateCheckoutSession(req.UserID, req.Email)
	if err != nil {
		// Log the detailed error for debugging
		log.Printf("LemonSqueezy checkout creation failed: %v", err)
		c.JSON(500, gin.H{"error": fmt.Sprintf("Failed to create checkout session: %v", err)})
		return
	}

	// Validate the URL before returning
	if checkoutURL == "" {
		c.JSON(500, gin.H{"error": "Empty checkout URL received"})
		return
	}

	c.JSON(200, CheckoutResponse{
		URL: checkoutURL,
	})
}
