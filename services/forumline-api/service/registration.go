package service

import (
	"context"
	"fmt"
	"net/url"

	"github.com/forumline/forumline/services/forumline-api/store"
)

// OAuthCredentials holds Zitadel OIDC client credentials.
type OAuthCredentials struct {
	ClientID     string
	ClientSecret string
}

// CreateZitadelOIDCApp creates a Zitadel OIDC application for a forum via the Management API.
func CreateZitadelOIDCApp(ctx context.Context, s *store.Store, forumID string, domain string) (*OAuthCredentials, error) {
	z, err := GetZitadelClient(ctx)
	if err != nil {
		return nil, fmt.Errorf("zitadel client: %w", err)
	}

	redirectURIs := []string{"https://" + domain + "/api/forumline/auth/callback"}
	clientID, clientSecret, err := z.CreateOIDCApp(ctx, "Forum: "+domain, redirectURIs)
	if err != nil {
		return nil, fmt.Errorf("create OIDC app for %s: %w", domain, err)
	}

	return &OAuthCredentials{ClientID: clientID, ClientSecret: clientSecret}, nil
}

// RegisterForumInput contains validated input for forum registration.
type RegisterForumInput struct {
	Domain       string
	Name         string
	APIBase      string
	WebBase      string
	Capabilities []string
	Description  *string
	Tags         []string
	RedirectURIs []string
}

// RegisterForumResult contains the outcome of a forum registration.
type RegisterForumResult struct {
	ForumID      string
	ClientID     string
	ClientSecret string
	Approved     bool
	Message      string
}

// RegisterForum handles the full forum registration flow: validation, quota check,
// domain conflict resolution, and new forum creation.
func (fs *ForumService) RegisterForum(ctx context.Context, userID string, input RegisterForumInput) (*RegisterForumResult, error) {
	if input.Domain == "" || input.Name == "" || input.APIBase == "" || input.WebBase == "" {
		return nil, &ValidationError{Msg: "domain, name, api_base, and web_base are required"}
	}
	if err := ValidateDomain(input.Domain); err != nil {
		return nil, &ValidationError{Msg: fmt.Sprintf("invalid domain: %v", err)}
	}
	for _, u := range []string{input.APIBase, input.WebBase} {
		if _, err := url.ParseRequestURI(u); err != nil {
			return nil, &ValidationError{Msg: fmt.Sprintf("invalid URL: %s", u)}
		}
	}

	count, err := fs.Store.CountForumsByOwner(ctx, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to check forum quota: %w", err)
	}
	if count >= 5 {
		return nil, &ForbiddenError{Msg: "Maximum of 5 forums per user"}
	}

	exists, _ := fs.Store.DomainExists(ctx, input.Domain)
	if exists {
		return nil, &ConflictError{Msg: "Forum with this domain is already registered"}
	}

	tags := NormalizeTags(input.Tags)
	forumID, err := fs.Store.RegisterForum(ctx, input.Domain, input.Name, input.APIBase, input.WebBase,
		input.Capabilities, input.Description, tags, userID)
	if err != nil {
		return nil, fmt.Errorf("failed to register forum: %w", err)
	}

	// Create Zitadel OIDC application for this forum
	creds, err := CreateZitadelOIDCApp(ctx, fs.Store, forumID, input.Domain)
	if err != nil {
		// Registration succeeds but without OIDC credentials — admin can create them later
		return &RegisterForumResult{
			ForumID:  forumID,
			Approved: false,
			Message:  "Forum registered. OIDC credentials must be created in Zitadel Console.",
		}, nil
	}

	return &RegisterForumResult{
		ForumID:      forumID,
		ClientID:     creds.ClientID,
		ClientSecret: creds.ClientSecret,
		Approved:     false,
		Message:      "Forum registered with Zitadel OIDC credentials.",
	}, nil
}

// EnsureOAuth is a placeholder for the old OAuth credential provisioning.
// With Zitadel, OIDC apps are created via the Zitadel Management API.
func (fs *ForumService) EnsureOAuth(ctx context.Context, domain string) (*OAuthCredentials, error) {
	if domain == "" {
		return nil, &ValidationError{Msg: "domain is required"}
	}
	forumID := fs.Store.GetForumIDByDomain(ctx, domain)
	if forumID == "" {
		return nil, &NotFoundError{Msg: "forum not found"}
	}

	return CreateZitadelOIDCApp(ctx, fs.Store, forumID, domain)
}
