package integrations

// GitHub adapter — https://docs.github.com/en/webhooks/using-webhooks/validating-webhook-deliveries
//
// Signature: X-Hub-Signature-256 header, "sha256=" + hex HMAC-SHA256 of the
// raw body with the webhook secret. (Verified from GitHub docs, including
// their test vector: secret "It's a Secret to Everybody", payload
// "Hello, World!" → 757107ea0eb2509fc211221cce984b8a37570b6d7586c22c46f4379c8b043e17.)
//
// Mapped events:
//   deployment_status (failure, production) → account.warning (rollback call)
//   workflow_run (failure, main branch)      → account.warning (CI escalation)

import (
	"net/http"
	"os"
)

// GitHubConfig configures the GitHub adapter.
type GitHubConfig struct {
	// Secret is the webhook secret. Falls back to GITHUB_WEBHOOK_SECRET.
	Secret string
}

func (c GitHubConfig) secret() string {
	if c.Secret != "" {
		return c.Secret
	}
	return os.Getenv("GITHUB_WEBHOOK_SECRET")
}

type githubAdapter struct{ cfg GitHubConfig }

func (githubAdapter) Platform() string { return "github" }

type githubPayload struct {
	Action     string `json:"action"`
	Deployment struct {
		ID          int64  `json:"id"`
		Environment string `json:"environment"`
	} `json:"deployment"`
	DeploymentStatus struct {
		State string `json:"state"`
	} `json:"deployment_status"`
	WorkflowRun struct {
		ID         int64  `json:"id"`
		Name       string `json:"name"`
		HeadBranch string `json:"head_branch"`
		Conclusion string `json:"conclusion"`
	} `json:"workflow_run"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Sender struct {
		Login string `json:"login"`
	} `json:"sender"`
}

func (a githubAdapter) Handle(r *http.Request, body []byte) ([]Event, error) {
	secret := a.cfg.secret()
	if secret == "" {
		return nil, ErrUnauthorized("GITHUB_WEBHOOK_SECRET not configured")
	}
	sig := r.Header.Get("X-Hub-Signature-256")
	if sig == "" {
		return nil, ErrUnauthorized("missing X-Hub-Signature-256 (no secret configured on the hook?)")
	}
	if !VerifyHexHMAC(secret, body, sig) {
		return nil, ErrUnauthorized("invalid github signature")
	}

	event := r.Header.Get("X-GitHub-Event")
	var p githubPayload
	if err := jsonUnmarshalStrict(body, &p); err != nil {
		return nil, err
	}

	// The customer is the engineer who triggered the event — mapped by
	// login in your user directory (business.Store).
	customerID := "gh_" + p.Sender.Login

	switch event {
	case "deployment_status":
		if p.DeploymentStatus.State != "failure" && p.DeploymentStatus.State != "error" {
			return nil, nil // only failures escalate
		}
		env := p.Deployment.Environment
		if env != "" && env != "production" {
			return nil, nil // only production pages anyone
		}
		return []Event{{
			ID:         "gh_dep_" + strconvFormat(p.Deployment.ID),
			Type:       "account.warning",
			CustomerID: customerID,
			Payload: payloadJSON(map[string]any{
				"reason": "production deployment failed",
				"detail": p.Repository.FullName + " deployment " + strconvFormat(p.Deployment.ID) + " to " + env,
			}),
		}}, nil
	case "workflow_run":
		if p.WorkflowRun.Conclusion != "failure" {
			return nil, nil
		}
		if p.WorkflowRun.HeadBranch != "main" && p.WorkflowRun.HeadBranch != "master" {
			return nil, nil
		}
		return []Event{{
			ID:         "gh_ci_" + strconvFormat(p.WorkflowRun.ID),
			Type:       "account.warning",
			CustomerID: customerID,
			Payload: payloadJSON(map[string]any{
				"reason": "CI failed on the default branch",
				"detail": p.Repository.FullName + " workflow " + p.WorkflowRun.Name,
			}),
		}}, nil
	default:
		return nil, nil
	}
}

// NewGitHub returns the GitHub adapter.
func NewGitHub(cfg GitHubConfig) Adapter { return githubAdapter{cfg: cfg} }
