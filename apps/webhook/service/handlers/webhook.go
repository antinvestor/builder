package handlers

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/pitabwire/frame/queue"
	"github.com/pitabwire/util"

	appconfig "github.com/antinvestor/builder/apps/webhook/config"
)

// Label represents a GitHub label.
type Label struct {
	Name string `json:"name"`
}

// WebhookHandler handles incoming GitHub webhooks.
type WebhookHandler struct {
	cfg   *appconfig.WebhookConfig
	queue queue.Manager
}

// NewWebhookHandler creates a new webhook handler.
func NewWebhookHandler(cfg *appconfig.WebhookConfig, qMan queue.Manager) *WebhookHandler {
	return &WebhookHandler{
		cfg:   cfg,
		queue: qMan,
	}
}

// HandleGitHubWebhook processes incoming GitHub webhook events.
func (h *WebhookHandler) HandleGitHubWebhook(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := util.Log(ctx)

	if r.Method != http.MethodPost {
		http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
		return
	}

	// Read body
	body, err := io.ReadAll(r.Body)
	if err != nil {
		log.WithError(err).Error("failed to read request body")
		http.Error(w, "Failed to read body", http.StatusBadRequest)
		return
	}
	defer util.CloseAndLogOnError(ctx, r.Body, "failed to close request body")

	// Verify signature if secret is configured
	if h.cfg.GitHubWebhookSecret != "" {
		signature := r.Header.Get("X-Hub-Signature-256")
		if !h.verifySignature(body, signature) {
			log.Warn("invalid webhook signature")
			http.Error(w, "Invalid signature", http.StatusUnauthorized)
			return
		}
	}

	// Get event type (using GitHub's header names - Go's Header.Get is case-insensitive)
	eventType := r.Header.Get("X-GitHub-Event")
	deliveryID := r.Header.Get("X-GitHub-Delivery")

	log.Info("received GitHub webhook",
		"event_type", eventType,
		"delivery_id", deliveryID,
	)

	// Process based on event type
	switch eventType {
	case "issues":
		h.handleIssueEvent(w, r, body)
	case "issue_comment":
		h.handleIssueCommentEvent(w, r, body)
	case "pull_request":
		h.handlePullRequestEvent(w, r, body)
	case "push":
		h.handlePushEvent(w, r, body)
	case "ping":
		h.handlePingEvent(w, r, body)
	default:
		log.Debug("ignoring unhandled event type", "event_type", eventType)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ignored","reason":"unhandled event type"}`))
	}
}

func (h *WebhookHandler) verifySignature(body []byte, signature string) bool {
	if signature == "" {
		return false
	}

	// Remove the "sha256=" prefix
	if !strings.HasPrefix(signature, "sha256=") {
		return false
	}
	signature = strings.TrimPrefix(signature, "sha256=")

	// Compute expected signature
	mac := hmac.New(sha256.New, []byte(h.cfg.GitHubWebhookSecret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	return hmac.Equal([]byte(expected), []byte(signature))
}

// IssueEvent represents a GitHub issue event payload.
type IssueEvent struct {
	Action string `json:"action"`
	Issue  struct {
		Number int     `json:"number"`
		Title  string  `json:"title"`
		Body   string  `json:"body"`
		State  string  `json:"state"`
		Labels []Label `json:"labels"`
		User   struct {
			Login string `json:"login"`
		} `json:"user"`
		HTMLURL string `json:"html_url"`
	} `json:"issue"`
	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
		SSHURL   string `json:"ssh_url"`
		HTMLURL  string `json:"html_url"`
		Private  bool   `json:"private"`
		Owner    struct {
			Login string `json:"login"`
		} `json:"owner"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
	Label Label `json:"label"`
}

func (h *WebhookHandler) handleIssueEvent(w http.ResponseWriter, r *http.Request, body []byte) {
	ctx := r.Context()
	log := util.Log(ctx)

	if !h.cfg.EnableIssueProcessing {
		log.Debug("issue processing disabled")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ignored","reason":"issue processing disabled"}`))
		return
	}

	var event IssueEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.WithError(err).Error("failed to parse issue event")
		http.Error(w, "Failed to parse event", http.StatusBadRequest)
		return
	}

	log.Info("processing issue event",
		"action", event.Action,
		"repo", event.Repository.FullName,
		"issue", event.Issue.Number,
		"title", event.Issue.Title,
	)

	// Check if repository is allowed
	if !h.isRepositoryAllowed(event.Repository.FullName) {
		log.Debug("repository not in allowed list", "repo", event.Repository.FullName)
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ignored","reason":"repository not allowed"}`))
		return
	}

	// Process based on action
	switch event.Action {
	case "opened", "labeled":
		// Check if auto-trigger label is present
		if h.hasAutoTriggerLabel(event.Issue.Labels) {
			if err := h.publishFeatureRequest(ctx, &event); err != nil {
				log.WithError(err).Error("failed to publish feature request")
				http.Error(w, "Failed to queue feature request", http.StatusInternalServerError)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusAccepted)
			_, _ = w.Write([]byte(`{"status":"accepted","message":"Feature request queued"}`))
			return
		}
	case "closed", "reopened", "edited":
		// Publish state change event
		if err := h.publishGitHubEvent(ctx, "issue", event.Action, body); err != nil {
			log.WithError(err).Error("failed to publish GitHub event")
		}
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"processed"}`))
}

// IssueCommentEvent represents a GitHub issue comment event.
type IssueCommentEvent struct {
	Action string `json:"action"`
	Issue  struct {
		Number      int     `json:"number"`
		Title       string  `json:"title"`
		Body        string  `json:"body"`
		Labels      []Label `json:"labels"`
		PullRequest *struct {
			URL string `json:"url"`
		} `json:"pull_request"`
	} `json:"issue"`
	Comment struct {
		ID   int64  `json:"id"`
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"comment"`
	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
		SSHURL   string `json:"ssh_url"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
}

func (h *WebhookHandler) handleIssueCommentEvent(w http.ResponseWriter, r *http.Request, body []byte) {
	ctx := r.Context()
	log := util.Log(ctx)

	var event IssueCommentEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.WithError(err).Error("failed to parse issue comment event")
		http.Error(w, "Failed to parse event", http.StatusBadRequest)
		return
	}

	log.Info("processing issue comment event",
		"action", event.Action,
		"repo", event.Repository.FullName,
		"issue", event.Issue.Number,
		"commenter", event.Comment.User.Login,
	)

	// Check for command triggers in comments
	if event.Action == "created" {
		h.processCommentCommands(ctx, &event)
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"processed"}`))
}

func (h *WebhookHandler) processCommentCommands(ctx context.Context, event *IssueCommentEvent) {
	log := util.Log(ctx)
	comment := strings.TrimSpace(event.Comment.Body)

	// Check for build trigger command
	if strings.HasPrefix(comment, "/build") || strings.HasPrefix(comment, "/auto-build") {
		log.Info("build command detected",
			"repo", event.Repository.FullName,
			"issue", event.Issue.Number,
			"requester", event.Comment.User.Login,
		)
		// Convert to feature request
		issueEvent := &IssueEvent{
			Action: "comment_triggered",
		}
		issueEvent.Issue.Number = event.Issue.Number
		issueEvent.Issue.Title = event.Issue.Title
		issueEvent.Issue.Body = event.Issue.Body
		// Copy labels from IssueCommentEvent to IssueEvent
		for _, l := range event.Issue.Labels {
			issueEvent.Issue.Labels = append(issueEvent.Issue.Labels, Label{Name: l.Name})
		}
		issueEvent.Repository.FullName = event.Repository.FullName
		issueEvent.Repository.CloneURL = event.Repository.CloneURL
		issueEvent.Repository.SSHURL = event.Repository.SSHURL

		if err := h.publishFeatureRequest(ctx, issueEvent); err != nil {
			log.WithError(err).Error("failed to publish feature request from comment")
		}
	}
}

// PullRequestEvent represents a GitHub pull request event.
type PullRequestEvent struct {
	Action string `json:"action"`
	Number int    `json:"number"`
	PR     struct {
		Number    int    `json:"number"`
		Title     string `json:"title"`
		Body      string `json:"body"`
		State     string `json:"state"`
		Merged    bool   `json:"merged"`
		MergeHash string `json:"merge_commit_sha"`
		Head      struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
		Base struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"base"`
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"pull_request"`
	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
		SSHURL   string `json:"ssh_url"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
}

func (h *WebhookHandler) handlePullRequestEvent(w http.ResponseWriter, r *http.Request, body []byte) {
	ctx := r.Context()
	log := util.Log(ctx)

	if !h.cfg.EnablePRProcessing {
		log.Debug("PR processing disabled")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ignored","reason":"PR processing disabled"}`))
		return
	}

	var event PullRequestEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.WithError(err).Error("failed to parse pull request event")
		http.Error(w, "Failed to parse event", http.StatusBadRequest)
		return
	}

	log.Info("processing pull request event",
		"action", event.Action,
		"repo", event.Repository.FullName,
		"pr", event.Number,
		"title", event.PR.Title,
	)

	// Publish state change event
	if err := h.publishGitHubEvent(ctx, "pull_request", event.Action, body); err != nil {
		log.WithError(err).Error("failed to publish GitHub event")
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"processed"}`))
}

// PushEvent represents a GitHub push event.
type PushEvent struct {
	Ref        string `json:"ref"`
	Before     string `json:"before"`
	After      string `json:"after"`
	Created    bool   `json:"created"`
	Deleted    bool   `json:"deleted"`
	Forced     bool   `json:"forced"`
	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
		SSHURL   string `json:"ssh_url"`
	} `json:"repository"`
	Pusher struct {
		Name  string `json:"name"`
		Email string `json:"email"`
	} `json:"pusher"`
	Commits []struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  struct {
			Name  string `json:"name"`
			Email string `json:"email"`
		} `json:"author"`
	} `json:"commits"`
}

func (h *WebhookHandler) handlePushEvent(w http.ResponseWriter, r *http.Request, body []byte) {
	ctx := r.Context()
	log := util.Log(ctx)

	if !h.cfg.EnablePushProcessing {
		log.Debug("push processing disabled")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ignored","reason":"push processing disabled"}`))
		return
	}

	var event PushEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.WithError(err).Error("failed to parse push event")
		http.Error(w, "Failed to parse event", http.StatusBadRequest)
		return
	}

	log.Info("processing push event",
		"repo", event.Repository.FullName,
		"ref", event.Ref,
		"commits", len(event.Commits),
	)

	// Publish state change event
	if err := h.publishGitHubEvent(ctx, "push", "pushed", body); err != nil {
		log.WithError(err).Error("failed to publish GitHub event")
	}

	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"processed"}`))
}

// PingEvent represents a GitHub ping event.
type PingEvent struct {
	Zen    string `json:"zen"`
	HookID int64  `json:"hook_id"`
	Hook   struct {
		Type   string   `json:"type"`
		ID     int64    `json:"id"`
		Name   string   `json:"name"`
		Active bool     `json:"active"`
		Events []string `json:"events"`
	} `json:"hook"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
}

func (h *WebhookHandler) handlePingEvent(w http.ResponseWriter, r *http.Request, body []byte) {
	ctx := r.Context()
	log := util.Log(ctx)

	var event PingEvent
	if err := json.Unmarshal(body, &event); err != nil {
		log.WithError(err).Error("failed to parse ping event")
		http.Error(w, "Failed to parse event", http.StatusBadRequest)
		return
	}

	log.Info("received ping event",
		"zen", event.Zen,
		"hook_id", event.HookID,
		"repo", event.Repository.FullName,
	)

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte(`{"status":"pong","message":"Webhook configured successfully"}`))
}

func (h *WebhookHandler) isRepositoryAllowed(repo string) bool {
	if h.cfg.AllowedRepositories == "" {
		return true // All repositories allowed
	}

	allowed := strings.Split(h.cfg.AllowedRepositories, ",")
	for _, r := range allowed {
		if strings.TrimSpace(r) == repo {
			return true
		}
	}
	return false
}

func (h *WebhookHandler) hasAutoTriggerLabel(labels []Label) bool {
	for _, label := range labels {
		if label.Name == h.cfg.AutoTriggerLabel {
			return true
		}
	}
	return false
}

// FeatureRequest represents a feature request to be published to the queue.
type FeatureRequest struct {
	ID            string            `json:"id"`
	Source        string            `json:"source"`
	Repository    string            `json:"repository"`
	RepositoryURL string            `json:"repository_url"`
	IssueNumber   int               `json:"issue_number,omitempty"`
	Title         string            `json:"title"`
	Description   string            `json:"description"`
	Labels        []string          `json:"labels,omitempty"`
	Requester     string            `json:"requester,omitempty"`
	Metadata      map[string]string `json:"metadata,omitempty"`
}

func (h *WebhookHandler) publishFeatureRequest(ctx context.Context, event *IssueEvent) error {
	log := util.Log(ctx)

	// Build labels list
	labels := make([]string, 0, len(event.Issue.Labels))
	for _, l := range event.Issue.Labels {
		labels = append(labels, l.Name)
	}

	request := &FeatureRequest{
		ID:            fmt.Sprintf("gh-%s-%d", event.Repository.FullName, event.Issue.Number),
		Source:        "github",
		Repository:    event.Repository.FullName,
		RepositoryURL: event.Repository.CloneURL,
		IssueNumber:   event.Issue.Number,
		Title:         event.Issue.Title,
		Description:   event.Issue.Body,
		Labels:        labels,
		Requester:     event.Issue.User.Login,
		Metadata: map[string]string{
			"github_url": event.Issue.HTMLURL,
			"action":     event.Action,
		},
	}

	data, marshalErr := json.Marshal(request)
	if marshalErr != nil {
		return fmt.Errorf("marshal feature request: %w", marshalErr)
	}

	publisher, pubErr := h.queue.GetPublisher(h.cfg.QueueFeatureRequestName)
	if pubErr != nil {
		return fmt.Errorf("get publisher %s: %w", h.cfg.QueueFeatureRequestName, pubErr)
	}

	if publishErr := publisher.Publish(ctx, data); publishErr != nil {
		return fmt.Errorf("publish feature request: %w", publishErr)
	}

	log.Info("published feature request",
		"id", request.ID,
		"repo", request.Repository,
		"issue", request.IssueNumber,
	)

	return nil
}

// GitHubEvent represents a GitHub event to be published for state tracking.
type GitHubEvent struct {
	Type       string          `json:"type"`
	Action     string          `json:"action"`
	Repository string          `json:"repository"`
	Timestamp  string          `json:"timestamp"`
	Payload    json.RawMessage `json:"payload"`
}

func (h *WebhookHandler) publishGitHubEvent(ctx context.Context, eventType, action string, payload []byte) error {
	log := util.Log(ctx)

	event := &GitHubEvent{
		Type:    eventType,
		Action:  action,
		Payload: payload,
	}

	data, marshalErr := json.Marshal(event)
	if marshalErr != nil {
		return fmt.Errorf("marshal GitHub event: %w", marshalErr)
	}

	publisher, pubErr := h.queue.GetPublisher(h.cfg.QueueGitHubEventName)
	if pubErr != nil {
		return fmt.Errorf("get publisher %s: %w", h.cfg.QueueGitHubEventName, pubErr)
	}

	if publishErr := publisher.Publish(ctx, data); publishErr != nil {
		return fmt.Errorf("publish GitHub event: %w", publishErr)
	}

	log.Debug("published GitHub event", "type", eventType, "action", action)

	return nil
}
