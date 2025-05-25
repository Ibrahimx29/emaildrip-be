package services

import (
	"database/sql"
	"time"
)

type EmailService struct {
	DB *sql.DB
}

type EmailRecord struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Original  string    `json:"original"`
	Rewritten string    `json:"rewritten"`
	Roast     string    `json:"roast"`
	Tone      string    `json:"tone"`
	RoastMode bool      `json:"roast_mode"`
	CreatedAt time.Time `json:"created_at"`
}

func NewEmailService(db *sql.DB) *EmailService {
	return &EmailService{DB: db}
}

func (es *EmailService) SaveEmail(email EmailRecord) error {
	query := `
		INSERT INTO emails (user_id, original, rewritten, roast, tone, roast_mode)
		VALUES ($1, $2, $3, $4, $5, $6)
	`
	_, err := es.DB.Exec(query, email.UserID, email.Original, email.Rewritten,
		email.Roast, email.Tone, email.RoastMode)
	return err
}

func (es *EmailService) GetUserEmails(userID string, limit int) ([]EmailRecord, error) {
	query := `
		SELECT id, user_id, original, rewritten, COALESCE(roast, ''), tone, roast_mode, created_at
		FROM emails
		WHERE user_id = $1
		ORDER BY created_at DESC
		LIMIT $2
	`

	rows, err := es.DB.Query(query, userID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var emails []EmailRecord
	for rows.Next() {
		var email EmailRecord
		err := rows.Scan(&email.ID, &email.UserID, &email.Original, &email.Rewritten,
			&email.Roast, &email.Tone, &email.RoastMode, &email.CreatedAt)
		if err != nil {
			return nil, err
		}
		emails = append(emails, email)
	}

	return emails, nil
}

func (es *EmailService) IncrementUsage(userID string) error {
	_, err := es.DB.Exec("SELECT increment_usage($1)", userID)
	return err
}

func (es *EmailService) GetUserUsage(userID string) (int, error) {
	var usage int
	err := es.DB.QueryRow("SELECT get_user_usage($1)", userID).Scan(&usage)
	return usage, err
}

func (es *EmailService) IsUserPro(userID string) (bool, error) {
	var isPro bool
	err := es.DB.QueryRow("SELECT is_user_pro($1)", userID).Scan(&isPro)
	return isPro, err
}

func (es *EmailService) CanUserMakeRequest(userID string) (bool, error) {
	isPro, err := es.IsUserPro(userID)
	if err != nil {
		return false, err
	}

	if isPro {
		return true, nil
	}

	usage, err := es.GetUserUsage(userID)
	if err != nil {
		return false, err
	}

	return usage < 5, nil // Free limit is 5
}
