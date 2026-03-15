package service

import (
	"context"
	"fmt"
	"log"
	"os"
	"sync"

	"github.com/zitadel/zitadel-go/v3/pkg/client"
	appv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/application/v2"
	projectv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/project/v2"
	userv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/user/v2"
	"github.com/zitadel/zitadel-go/v3/pkg/zitadel"
)

// ZitadelClient wraps the Zitadel Management API client.
// Initialized lazily on first use via InitZitadelClient or automatically.
type ZitadelClient struct {
	api       *client.Client
	projectID string
}

var (
	zitadelOnce   sync.Once
	zitadelClient *ZitadelClient
	zitadelErr    error
)

// GetZitadelClient returns the singleton Zitadel API client.
// Initializes on first call using ZITADEL_URL and ZITADEL_SERVICE_USER_PAT env vars.
func GetZitadelClient(ctx context.Context) (*ZitadelClient, error) {
	zitadelOnce.Do(func() {
		zitadelClient, zitadelErr = initZitadelClient(ctx)
	})
	return zitadelClient, zitadelErr
}

func initZitadelClient(ctx context.Context) (*ZitadelClient, error) {
	zitadelURL := os.Getenv("ZITADEL_URL")
	pat := os.Getenv("ZITADEL_SERVICE_USER_PAT")
	if zitadelURL == "" || pat == "" {
		return nil, fmt.Errorf("ZITADEL_URL and ZITADEL_SERVICE_USER_PAT are required")
	}

	api, err := client.New(ctx,
		zitadel.New(zitadelURL),
		client.WithAuth(client.PAT(pat)),
	)
	if err != nil {
		return nil, fmt.Errorf("init zitadel client: %w", err)
	}

	// Ensure the "Forumline" project exists (idempotent)
	projectID, err := ensureProject(ctx, api)
	if err != nil {
		_ = api.Close()
		return nil, fmt.Errorf("ensure project: %w", err)
	}

	return &ZitadelClient{api: api, projectID: projectID}, nil
}

// ensureProject creates or finds the "Forumline" project.
func ensureProject(ctx context.Context, api *client.Client) (string, error) {
	// Check env var override first
	if id := os.Getenv("ZITADEL_PROJECT_ID"); id != "" {
		return id, nil
	}

	// Try to create the project (will fail if it already exists by name — that's fine)
	resp, err := api.ProjectServiceV2().CreateProject(ctx, &projectv2.CreateProjectRequest{
		Name: "Forumline",
	})
	if err != nil {
		// Project may already exist — list and find it
		listResp, listErr := api.ProjectServiceV2().ListProjects(ctx, &projectv2.ListProjectsRequest{})
		if listErr != nil {
			return "", fmt.Errorf("list projects: %w", listErr)
		}
		for _, p := range listResp.GetProjects() {
			if p.GetName() == "Forumline" {
				log.Printf("[Zitadel] Using existing project: %q", p.GetProjectId()) // #nosec G706 -- project ID from trusted Zitadel API
				return p.GetProjectId(), nil
			}
		}
		return "", fmt.Errorf("create project: %w", err)
	}

	log.Printf("[Zitadel] Created project: %q", resp.GetProjectId()) // #nosec G706 -- project ID from trusted Zitadel API
	return resp.GetProjectId(), nil
}

// CreateOIDCApp creates a Zitadel OIDC application for a forum.
// Returns the client_id and client_secret.
func (z *ZitadelClient) CreateOIDCApp(ctx context.Context, name string, redirectURIs []string) (clientID string, clientSecret string, err error) {
	resp, err := z.api.ApplicationServiceV2().CreateApplication(ctx, &appv2.CreateApplicationRequest{
		ProjectId: z.projectID,
		Name:      name,
		ApplicationType: &appv2.CreateApplicationRequest_OidcConfiguration{
			OidcConfiguration: &appv2.CreateOIDCApplicationRequest{
				ApplicationType: appv2.OIDCApplicationType_OIDC_APP_TYPE_WEB,
				AuthMethodType:  appv2.OIDCAuthMethodType_OIDC_AUTH_METHOD_TYPE_NONE,
				GrantTypes:      []appv2.OIDCGrantType{appv2.OIDCGrantType_OIDC_GRANT_TYPE_AUTHORIZATION_CODE},
				ResponseTypes:   []appv2.OIDCResponseType{appv2.OIDCResponseType_OIDC_RESPONSE_TYPE_CODE},
				RedirectUris:    redirectURIs,
				AccessTokenType: appv2.OIDCTokenType_OIDC_TOKEN_TYPE_JWT,
			},
		},
	})
	if err != nil {
		return "", "", fmt.Errorf("create OIDC app: %w", err)
	}

	oidcCfg := resp.GetOidcConfiguration()
	return oidcCfg.GetClientId(), oidcCfg.GetClientSecret(), nil
}

// DeleteUser deletes a user from Zitadel.
func (z *ZitadelClient) DeleteUser(ctx context.Context, userID string) error {
	_, err := z.api.UserServiceV2().DeleteUser(ctx, &userv2.DeleteUserRequest{
		UserId: userID,
	})
	return err
}

// Close closes the Zitadel gRPC connection.
func (z *ZitadelClient) Close() {
	if z.api != nil {
		_ = z.api.Close()
	}
}
