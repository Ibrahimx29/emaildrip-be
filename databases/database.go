package databases

import (
	"database/sql"
	"fmt"
	"os"

	_ "github.com/lib/pq"
)

func InitDB() *sql.DB {
	dbURL := os.Getenv("DATABASE_URL")
	if dbURL == "" {
		// Construct URL from individual components if DATABASE_URL not provided
		dbURL = fmt.Sprintf("postgres://%s:%s@%s:%s/%s?sslmode=require",
			os.Getenv("DB_USER"),
			os.Getenv("DB_PASSWORD"),
			os.Getenv("DB_HOST"),
			os.Getenv("DB_PORT"),
			os.Getenv("DB_NAME"),
		)
	}

	db, err := sql.Open("postgres", dbURL)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to database: %v", err))
	}

	if err := db.Ping(); err != nil {
		panic(fmt.Sprintf("Failed to ping database: %v", err))
	}

	return db
}
