package main

import (
	"emaildrip-be/databases"
	"emaildrip-be/handlers"
	"emaildrip-be/services"
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	// Load environment variables
	// if err := godotenv.Load(); err != nil {
	// 	log.Println("No .env file found")
	// }

	// Initialize database connection
	db := databases.InitDB()
	defer db.Close()

	// Initialize services
	aiService := services.NewAIService(os.Getenv("OPENROUTER_API_KEY"))
	emailService := services.NewEmailService(db)
	lemonSqueezyService := services.NewLemonSqueezyService(
		os.Getenv("LEMONSQUEEZY_API_KEY"),
		os.Getenv("LEMONSQUEEZY_WEBHOOK_SECRET"),
		db,
	)

	// Initialize handlers
	handlers := &handlers.Handlers{
		AI:           aiService,
		Email:        emailService,
		LemonSqueezy: lemonSqueezyService,
	}

	// Setup Gin router
	r := gin.Default()

	// CORS middleware
	r.Use(cors.New(cors.Config{
		AllowOrigins:     []string{os.Getenv("ALLOWED_ORIGINS")},
		AllowMethods:     []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Origin", "Content-Type", "Accept", "Authorization", "X-Signature"},
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
		api.POST("/checkout", handlers.CreateCheckout)
		api.POST("/lemonsqueezy/webhook", handlers.LemonSqueezyWebhook)
	}

	// Start server
	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Server starting on port %s", port)
	r.Run(":" + port)
}
