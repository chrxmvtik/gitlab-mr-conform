package gitlab

import (
	"encoding/json"
	"fmt"
)

type WebhookEvent struct {
	ObjectKind       string           `json:"object_kind"`
	EventType        string           `json:"event_type"`
	User             User             `json:"user"`
	Project          Project          `json:"project"`
	ObjectAttributes ObjectAttributes `json:"object_attributes"`
}

type User struct {
	ID       int    `json:"id"`
	Name     string `json:"name"`
	Username string `json:"username"`
	Email    string `json:"email"`
}

type Project struct {
	ID                int    `json:"id"`
	Name              string `json:"name"`
	Description       string `json:"description"`
	WebURL            string `json:"web_url"`
	PathWithNamespace string `json:"path_with_namespace"`
}

type ObjectAttributes struct {
	ID           int    `json:"id"`
	IID          int    `json:"iid"`
	Title        string `json:"title"`
	Description  string `json:"description"`
	State        string `json:"state"`
	MergeStatus  string `json:"merge_status"`
	SourceBranch string `json:"source_branch"`
	TargetBranch string `json:"target_branch"`
	LastCommit   Commit `json:"last_commit"`
	Action       string `json:"action"`
}

type Commit struct {
	ID      string `json:"id"`
	Message string `json:"message"`
	Author  Author `json:"author"`
}

type Author struct {
	Name  string `json:"name"`
	Email string `json:"email"`
}

func ParseWebhookEvent(data []byte) (*WebhookEvent, error) {
	var event WebhookEvent
	if err := json.Unmarshal(data, &event); err != nil {
		return nil, fmt.Errorf("failed to parse webhook event: %w", err)
	}
	return &event, nil
}

func (e *WebhookEvent) IsMergeRequestEvent() bool {
	return e.ObjectKind == "merge_request"
}

func (e *WebhookEvent) IsRelevantAction() bool {
	relevantActions := map[string]bool{
		"open":   true,
		"reopen": true,
		"update": true,
	}
	return relevantActions[e.ObjectAttributes.Action]
}
