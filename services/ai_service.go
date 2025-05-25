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
	systemPrompt := fmt.Sprintf(`You are an expert email writer. Rewrite the given email in a %s tone. 

Guidelines:
- Polite: Professional, courteous, and respectful
- Funny: Add appropriate humor while maintaining professionalism
- Karen: Demanding, entitled, and dramatic (for fun)
- Direct: Straight to the point, no fluff

Keep the core message intact but improve the tone, grammar, and clarity. Return only the rewritten email.`, tone)

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
