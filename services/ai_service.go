package services

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
)

type AIService struct {
	APIKey  string
	BaseURL string
}

type OpenRouterRequest struct {
	Model    string    `json:"model"`
	Messages []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type OpenRouterResponse struct {
	Choices []Choice `json:"choices"`
}

type Choice struct {
	Message Message `json:"message"`
}

func NewAIService(apiKey string) *AIService {
	return &AIService{
		APIKey:  apiKey,
		BaseURL: "https://openrouter.ai/api/v1/chat/completions",
	}
}

func (ai *AIService) RewriteEmail(email, tone string) (string, error) {
	var toneGuidelines string

	switch tone {
	case "Polite":
		toneGuidelines = "Use a professional, respectful, and courteous tone."
	case "Funny":
		toneGuidelines = "Add light humor while keeping the message clear and professional."
	case "Direct":
		toneGuidelines = "Make the message concise, clear, and to the point. Avoid fluff."
	case "Karen":
		toneGuidelines = "Write in an exaggeratedly demanding, entitled, and dramatic tone. Over-the-top but still readable."
	default:
		toneGuidelines = "Write in a clear and professional tone."
	}

	systemPrompt := fmt.Sprintf(`You are an expert email writer. Rewrite the email below using the following tone guideline:

	%s

	Keep the core message intact. Improve tone, grammar, and clarity. Return only the rewritten email.`, toneGuidelines)

	return ai.callOpenRouter(systemPrompt, email, "mistralai/mixtral-8x7b-instruct")
}

func (ai *AIService) RoastEmail(email string) (string, error) {
	systemPrompt := `You are a witty email critic. Roast this email in a funny but not mean-spirited way. Point out awkward phrasing, unclear messages, or funny quirks. Keep it light-hearted and constructive. Return only the roast.`

	return ai.callOpenRouter(systemPrompt, email, "mistralai/mixtral-8x7b-instruct")
}

func (ai *AIService) callOpenRouter(systemPrompt, userMessage, model string) (string, error) {
	reqBody := OpenRouterRequest{
		Model: model,
		Messages: []Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userMessage},
		},
	}

	jsonData, err := json.Marshal(reqBody)
	if err != nil {
		return "", err
	}

	req, err := http.NewRequest("POST", ai.BaseURL, bytes.NewBuffer(jsonData))
	if err != nil {
		return "", err
	}

	req.Header.Set("Authorization", "Bearer "+ai.APIKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	var response OpenRouterResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", err
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("no response from AI")
	}

	return response.Choices[0].Message.Content, nil
}
