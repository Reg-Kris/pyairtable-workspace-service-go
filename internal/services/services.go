package services

import (
	"context"
	"errors"

	"go.uber.org/zap"

	"github.com/Reg-Kris/pyairtable-workspace-service/internal/config"
	"github.com/Reg-Kris/pyairtable-workspace-service/internal/models"
	"github.com/Reg-Kris/pyairtable-workspace-service/internal/repositories"
)

// Common errors
var (
	ErrWorkspaceNotFound    = errors.New("workspace not found")
	ErrProjectNotFound      = errors.New("project not found")
	ErrAirtableBaseNotFound = errors.New("airtable base not found")
	ErrUnauthorized         = errors.New("unauthorized")
	ErrQuotaExceeded        = errors.New("quota exceeded")
	ErrInvalidInput         = errors.New("invalid input")
)

// WorkspaceService interface
type WorkspaceService interface {
	CreateWorkspace(ctx context.Context, tenantID, userID string, req *models.CreateWorkspaceRequest) (*models.Workspace, error)
	GetWorkspace(ctx context.Context, workspaceID, userID string) (*models.Workspace, error)
	UpdateWorkspace(ctx context.Context, workspaceID, userID string, req *models.UpdateWorkspaceRequest) (*models.Workspace, error)
	DeleteWorkspace(ctx context.Context, workspaceID, userID string) error
	ListWorkspaces(ctx context.Context, filter *models.WorkspaceFilter, userID string) (*models.WorkspaceListResponse, error)
	GetWorkspaceStats(ctx context.Context, tenantID, userID string) (*models.WorkspaceStats, error)
	CheckUserAccess(ctx context.Context, workspaceID, userID string, requiredRole models.WorkspaceMemberRole) error
}

// ProjectService interface
type ProjectService interface {
	CreateProject(ctx context.Context, workspaceID, userID string, req *models.CreateProjectRequest) (*models.Project, error)
	GetProject(ctx context.Context, projectID, userID string) (*models.Project, error)
	UpdateProject(ctx context.Context, projectID, userID string, req *models.UpdateProjectRequest) (*models.Project, error)
	DeleteProject(ctx context.Context, projectID, userID string) error
	ListProjects(ctx context.Context, filter *models.ProjectFilter, userID string) (*models.ProjectListResponse, error)
}

// AirtableBaseService interface
type AirtableBaseService interface {
	ConnectBase(ctx context.Context, projectID, userID string, req *models.CreateAirtableBaseRequest) (*models.AirtableBase, error)
	GetBase(ctx context.Context, baseID, userID string) (*models.AirtableBase, error)
	UpdateBase(ctx context.Context, baseID, userID string, req *models.UpdateAirtableBaseRequest) (*models.AirtableBase, error)
	DisconnectBase(ctx context.Context, baseID, userID string) error
	ListBases(ctx context.Context, filter *models.AirtableBaseFilter, userID string) (*models.AirtableBaseListResponse, error)
	UpdateSyncStatus(ctx context.Context, baseID string) error
}

// MemberService interface
type MemberService interface {
	AddMember(ctx context.Context, workspaceID, userID string, req *models.AddWorkspaceMemberRequest) (*models.WorkspaceMember, error)
	UpdateMemberRole(ctx context.Context, workspaceID, memberUserID, userID string, req *models.UpdateWorkspaceMemberRequest) error
	RemoveMember(ctx context.Context, workspaceID, memberUserID, userID string) error
	ListMembers(ctx context.Context, workspaceID, userID string, page, pageSize int) (*models.WorkspaceMemberListResponse, error)
	GetUserWorkspaces(ctx context.Context, userID string) ([]*models.Workspace, error)
}

// AuditService interface
type AuditService interface {
	LogAction(ctx context.Context, workspaceID, userID, action, resourceType, resourceID string, changes map[string]interface{}) error
	GetAuditLogs(ctx context.Context, filter *models.AuditLogFilter, userID string) (*models.AuditLogListResponse, error)
	CleanupOldLogs(ctx context.Context, days int) error
}

// Services aggregates all service interfaces
type Services struct {
	Workspace    WorkspaceService
	Project      ProjectService
	AirtableBase AirtableBaseService
	Member       MemberService
	Audit        AuditService

	config *config.Config
	logger *zap.Logger
	repos  *repositories.Repositories
}

// New creates a new Services instance
func New(repos *repositories.Repositories, config *config.Config, logger *zap.Logger) *Services {
	// Create audit service first as other services depend on it
	auditService := NewAuditService(repos, logger)
	
	return &Services{
		Workspace:    NewWorkspaceService(repos, config, logger, auditService),
		Project:      NewProjectService(repos, config, logger, auditService),
		AirtableBase: NewAirtableBaseService(repos, config, logger, auditService),
		Member:       NewMemberService(repos, config, logger, auditService),
		Audit:        auditService,
		config:       config,
		logger:       logger,
		repos:        repos,
	}
}