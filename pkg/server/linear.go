package server

import (
	"encoding/json"
	"time"
)

type LinearWebhook struct {
	Action           string          `json:"action"`
	Type             string          `json:"type"`
	CreatedAt        time.Time       `json:"createdAt"`
	Data             json.RawMessage `json:"data"`
	URL              string          `json:"url,omitempty"`
	OrganizationID   string          `json:"organizationId"`
	WebhookTimestamp int64           `json:"webhookTimestamp"`
	WebhookID        string          `json:"webhookId"`
}

type LinearIssue struct {
	ID          string     `json:"id"`
	Identifier  string     `json:"identifier"`
	Title       string     `json:"title"`
	Description string     `json:"description"`
	Priority    int        `json:"priority"`
	URL         string     `json:"url"`
	BranchName  string     `json:"branchName"`
	Number      int        `json:"number"`
	CreatedAt   time.Time  `json:"createdAt"`
	UpdatedAt   time.Time  `json:"updatedAt"`
	StartedAt   *time.Time `json:"startedAt,omitempty"`
	CompletedAt *time.Time `json:"completedAt,omitempty"`
	CanceledAt  *time.Time `json:"canceledAt,omitempty"`
	DueDate     *string    `json:"dueDate,omitempty"`
	TeamID      string     `json:"teamId"`
	ProjectID   *string    `json:"projectId,omitempty"`
	CycleID     *string    `json:"cycleId,omitempty"`
	StateID     string     `json:"stateId"`
	CreatorID   *string    `json:"creatorId,omitempty"`
	AssigneeID  *string    `json:"assigneeId,omitempty"`
	State       *LinearIssueState `json:"state,omitempty"`
	Team        *LinearTeam       `json:"team,omitempty"`
	Assignee    *LinearUser       `json:"assignee,omitempty"`
	Labels      []LinearLabel     `json:"labels,omitempty"`
}

type LinearIssueState struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
	Type  string `json:"type"`
}

type LinearTeam struct {
	ID   string `json:"id"`
	Key  string `json:"key"`
	Name string `json:"name"`
}

type LinearUser struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
}

type LinearLabel struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}
