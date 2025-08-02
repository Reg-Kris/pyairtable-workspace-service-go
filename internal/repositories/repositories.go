package repositories

import (
	"context"
	"errors"
	"time"

	"github.com/redis/go-redis/v9"
	"gorm.io/gorm"
	"go.uber.org/zap"

	"github.com/Reg-Kris/pyairtable-workspace-service/internal/models"
)

// Common errors
var (
	ErrWorkspaceNotFound     = errors.New("workspace not found")
	ErrProjectNotFound       = errors.New("project not found")
	ErrAirtableBaseNotFound  = errors.New("airtable base not found")
	ErrMemberNotFound        = errors.New("member not found")
	ErrDuplicateWorkspace    = errors.New("workspace with this name already exists")
	ErrDuplicateProject      = errors.New("project with this name already exists")
	ErrDuplicateAirtableBase = errors.New("airtable base already connected")
	ErrDuplicateMember       = errors.New("member already exists in workspace")
	ErrCannotDeleteOwner     = errors.New("cannot remove workspace owner")
	ErrLastOwner             = errors.New("cannot remove the last owner")
)

// WorkspaceRepository interface
type WorkspaceRepository interface {
	Create(ctx context.Context, workspace *models.Workspace) error
	GetByID(ctx context.Context, id string) (*models.Workspace, error)
	GetByTenantAndName(ctx context.Context, tenantID, name string) (*models.Workspace, error)
	Update(ctx context.Context, workspace *models.Workspace) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter *models.WorkspaceFilter) ([]*models.Workspace, int64, error)
	GetStats(ctx context.Context, tenantID string) (*models.WorkspaceStats, error)
}

// ProjectRepository interface
type ProjectRepository interface {
	Create(ctx context.Context, project *models.Project) error
	GetByID(ctx context.Context, id string) (*models.Project, error)
	GetByWorkspaceAndName(ctx context.Context, workspaceID, name string) (*models.Project, error)
	Update(ctx context.Context, project *models.Project) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter *models.ProjectFilter) ([]*models.Project, int64, error)
	CountByWorkspace(ctx context.Context, workspaceID string) (int64, error)
}

// AirtableBaseRepository interface
type AirtableBaseRepository interface {
	Create(ctx context.Context, base *models.AirtableBase) error
	GetByID(ctx context.Context, id string) (*models.AirtableBase, error)
	GetByProjectAndBaseID(ctx context.Context, projectID, baseID string) (*models.AirtableBase, error)
	Update(ctx context.Context, base *models.AirtableBase) error
	Delete(ctx context.Context, id string) error
	List(ctx context.Context, filter *models.AirtableBaseFilter) ([]*models.AirtableBase, int64, error)
	UpdateSyncTime(ctx context.Context, id string, syncTime time.Time) error
}

// WorkspaceMemberRepository interface
type WorkspaceMemberRepository interface {
	Add(ctx context.Context, member *models.WorkspaceMember) error
	GetByWorkspaceAndUser(ctx context.Context, workspaceID, userID string) (*models.WorkspaceMember, error)
	UpdateRole(ctx context.Context, workspaceID, userID string, role models.WorkspaceMemberRole) error
	Remove(ctx context.Context, workspaceID, userID string) error
	List(ctx context.Context, workspaceID string, page, pageSize int) ([]*models.WorkspaceMember, int64, error)
	CountOwners(ctx context.Context, workspaceID string) (int64, error)
	IsLastOwner(ctx context.Context, workspaceID, userID string) (bool, error)
}

// AuditLogRepository interface
type AuditLogRepository interface {
	Create(ctx context.Context, log *models.WorkspaceAuditLog) error
	List(ctx context.Context, filter *models.AuditLogFilter) ([]*models.WorkspaceAuditLog, int64, error)
	DeleteOlderThan(ctx context.Context, days int) error
}

// CacheRepository interface
type CacheRepository interface {
	SetWorkspace(ctx context.Context, workspace *models.Workspace) error
	GetWorkspace(ctx context.Context, id string) (*models.Workspace, error)
	DeleteWorkspace(ctx context.Context, id string) error
	SetProject(ctx context.Context, project *models.Project) error
	GetProject(ctx context.Context, id string) (*models.Project, error)
	DeleteProject(ctx context.Context, id string) error
	InvalidateWorkspaceCache(ctx context.Context, workspaceID string) error
	SetUserWorkspaces(ctx context.Context, userID string, workspaceIDs []string) error
	GetUserWorkspaces(ctx context.Context, userID string) ([]string, error)
}

// Repositories aggregates all repository interfaces
type Repositories struct {
	Workspace     WorkspaceRepository
	Project       ProjectRepository
	AirtableBase  AirtableBaseRepository
	Member        WorkspaceMemberRepository
	AuditLog      AuditLogRepository
	Cache         CacheRepository
	
	db     *gorm.DB
	redis  *redis.Client
	logger *zap.Logger
}

// New creates a new Repositories instance
func New(db *gorm.DB, redis *redis.Client, logger *zap.Logger) *Repositories {
	return &Repositories{
		Workspace:    NewWorkspaceRepository(db, logger),
		Project:      NewProjectRepository(db, logger),
		AirtableBase: NewAirtableBaseRepository(db, logger),
		Member:       NewWorkspaceMemberRepository(db, logger),
		AuditLog:     NewAuditLogRepository(db, logger),
		Cache:        NewCacheRepository(redis, logger),
		db:           db,
		redis:        redis,
		logger:       logger,
	}
}

// BeginTx starts a new transaction
func (r *Repositories) BeginTx(ctx context.Context) *gorm.DB {
	return r.db.WithContext(ctx).Begin()
}

// AutoMigrate runs database migrations
func (r *Repositories) AutoMigrate() error {
	return r.db.AutoMigrate(
		&models.Workspace{},
		&models.Project{},
		&models.AirtableBase{},
		&models.WorkspaceMember{},
		&models.WorkspaceAuditLog{},
	)
}