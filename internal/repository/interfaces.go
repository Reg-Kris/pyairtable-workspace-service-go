package repository

import (
	"context"
	
	"github.com/Reg-Kris/pyairtable-workspace-service/internal/models"
)

// WorkspaceRepository defines the interface for workspace data operations
type WorkspaceRepository interface {
	Create(ctx context.Context, workspace *models.Workspace) error
	FindByID(ctx context.Context, id string) (*models.Workspace, error)
	FindByTenantID(ctx context.Context, tenantID string, filter *models.WorkspaceFilter) ([]*models.Workspace, int64, error)
	Update(ctx context.Context, workspace *models.Workspace) error
	SoftDelete(ctx context.Context, id string) error
	List(ctx context.Context, filter *models.WorkspaceFilter) ([]*models.Workspace, int64, error)
	GetStats(ctx context.Context, tenantID string) (*models.WorkspaceStats, error)
	CountByTenant(ctx context.Context, tenantID string) (int64, error)
}

// ProjectRepository defines the interface for project data operations
type ProjectRepository interface {
	Create(ctx context.Context, project *models.Project) error
	FindByID(ctx context.Context, id string) (*models.Project, error)
	FindByWorkspaceID(ctx context.Context, workspaceID string, filter *models.ProjectFilter) ([]*models.Project, int64, error)
	Update(ctx context.Context, project *models.Project) error
	SoftDelete(ctx context.Context, id string) error
	UpdateStatus(ctx context.Context, id string, status string) error
	List(ctx context.Context, filter *models.ProjectFilter) ([]*models.Project, int64, error)
	CountByWorkspace(ctx context.Context, workspaceID string) (int64, error)
	CountByStatus(ctx context.Context, workspaceID string, status string) (int64, error)
}

// AirtableBaseRepository defines the interface for Airtable base data operations
type AirtableBaseRepository interface {
	Create(ctx context.Context, base *models.AirtableBase) error
	FindByID(ctx context.Context, id string) (*models.AirtableBase, error)
	FindByProjectID(ctx context.Context, projectID string, filter *models.AirtableBaseFilter) ([]*models.AirtableBase, int64, error)
	FindByBaseID(ctx context.Context, baseID string) (*models.AirtableBase, error)
	Update(ctx context.Context, base *models.AirtableBase) error
	Delete(ctx context.Context, id string) error
	UpdateSyncStatus(ctx context.Context, id string, syncEnabled bool) error
	UpdateLastSync(ctx context.Context, id string) error
	List(ctx context.Context, filter *models.AirtableBaseFilter) ([]*models.AirtableBase, int64, error)
	CountByProject(ctx context.Context, projectID string) (int64, error)
}

// WorkspaceMemberRepository defines the interface for workspace member data operations
type WorkspaceMemberRepository interface {
	Create(ctx context.Context, member *models.WorkspaceMember) error
	FindByWorkspaceAndUser(ctx context.Context, workspaceID, userID string) (*models.WorkspaceMember, error)
	FindByWorkspaceID(ctx context.Context, workspaceID string, page, pageSize int) ([]*models.WorkspaceMember, int64, error)
	FindByUserID(ctx context.Context, userID string) ([]*models.WorkspaceMember, error)
	Update(ctx context.Context, member *models.WorkspaceMember) error
	Delete(ctx context.Context, workspaceID, userID string) error
	List(ctx context.Context, workspaceID string) ([]*models.WorkspaceMember, error)
	CountByWorkspace(ctx context.Context, workspaceID string) (int64, error)
	CountByRole(ctx context.Context, workspaceID string, role models.WorkspaceMemberRole) (int64, error)
}

// AuditLogRepository defines the interface for audit log data operations
type AuditLogRepository interface {
	Create(ctx context.Context, log *models.WorkspaceAuditLog) error
	FindByID(ctx context.Context, id string) (*models.WorkspaceAuditLog, error)
	FindByWorkspaceID(ctx context.Context, workspaceID string, filter *models.AuditLogFilter) ([]*models.WorkspaceAuditLog, int64, error)
	FindByUserID(ctx context.Context, userID string, filter *models.AuditLogFilter) ([]*models.WorkspaceAuditLog, int64, error)
	List(ctx context.Context, filter *models.AuditLogFilter) ([]*models.WorkspaceAuditLog, int64, error)
	DeleteOldLogs(ctx context.Context, olderThan int) error // Delete logs older than N days
}

// CacheRepository defines the interface for caching operations
type CacheRepository interface {
	// Workspace cache operations
	GetWorkspace(ctx context.Context, id string) (*models.Workspace, error)
	SetWorkspace(ctx context.Context, workspace *models.Workspace, ttl int) error
	InvalidateWorkspace(ctx context.Context, id string) error
	InvalidateTenantWorkspaces(ctx context.Context, tenantID string) error
	
	// Project cache operations
	GetProject(ctx context.Context, id string) (*models.Project, error)
	SetProject(ctx context.Context, project *models.Project, ttl int) error
	InvalidateProject(ctx context.Context, id string) error
	InvalidateWorkspaceProjects(ctx context.Context, workspaceID string) error
	
	// Stats cache operations
	GetStats(ctx context.Context, key string) (*models.WorkspaceStats, error)
	SetStats(ctx context.Context, key string, stats *models.WorkspaceStats, ttl int) error
	InvalidateStats(ctx context.Context, key string) error
	
	// List cache operations
	GetWorkspaceList(ctx context.Context, key string) ([]*models.Workspace, error)
	SetWorkspaceList(ctx context.Context, key string, workspaces []*models.Workspace, ttl int) error
	GetProjectList(ctx context.Context, key string) ([]*models.Project, error)
	SetProjectList(ctx context.Context, key string, projects []*models.Project, ttl int) error
}