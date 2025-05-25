package main

import (
	"email-drip-be/databases"
	"email-drip-be/handlers"
	"email-drip-be/services"
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
)

func main() {
	// Load environment variables
	if err := godotenv.Load(); err != nil {
		log.Println("No .env file found")
	}

	// Initialize database connection
	db := databases.InitDB()
	defer db.Close()

	// Initialize services
	aiService := services.NewAIService(os.Getenv("OPENROUTER_API_KEY"))
	emailService := services.NewEmailService(db)
	stripeService := services.NewStripeService(os.Getenv("STRIPE_SECRET_KEY"))

	// Initialize handlers
	handlers := &handlers.Handlers{
		AI:     aiService,
		Email:  emailService,
		Stripe: stripeService,
	}

	// Setup Gin router
	r := gin.Default()

	// CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", "https://your-frontend.vercel.app"},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization"},
		AllowCredentials: true,
	}))

	// Health check
	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "healthy"})
	})

	// API routes
	api := r.Group("/api")
	{
		api.POST("/rewrite", handlers.RewriteEmail)
		api.GET("/usage/:user_id", handlers.GetUsage)
		api.GET("/emails/:user_id", handlers.GetUserEmails)
		api.POST("/stripe/webhook", handlers.StripeWebhook)
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	r.Run(":" + port)
}
