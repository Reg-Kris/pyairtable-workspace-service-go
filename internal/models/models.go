package models

import (
	"time"
	"encoding/json"
	"database/sql/driver"
	"fmt"

	"gorm.io/gorm"
)

// BaseModel contains common fields for all models
type BaseModel struct {
	ID        string         `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	CreatedAt time.Time      `json:"created_at"`
	UpdatedAt time.Time      `json:"updated_at"`
	DeletedAt gorm.DeletedAt `gorm:"index" json:"deleted_at,omitempty"`
}

// JSONMap represents a JSON map field
type JSONMap map[string]interface{}

// Scan implements the sql.Scanner interface for JSONMap
func (j *JSONMap) Scan(value interface{}) error {
	if value == nil {
		*j = make(JSONMap)
		return nil
	}
	
	switch v := value.(type) {
	case []byte:
		return json.Unmarshal(v, j)
	case string:
		return json.Unmarshal([]byte(v), j)
	default:
		return fmt.Errorf("cannot scan %T into JSONMap", value)
	}
}

// Value implements the driver.Valuer interface for JSONMap
func (j JSONMap) Value() (driver.Value, error) {
	if j == nil {
		return nil, nil
	}
	return json.Marshal(j)
}

// Workspace represents a workspace in the system
type Workspace struct {
	BaseModel
	TenantID    string  `gorm:"size:255;not null;index" json:"tenant_id"`
	Name        string  `gorm:"size:255;not null" json:"name"`
	Description string  `gorm:"type:text" json:"description"`
	Settings    JSONMap `gorm:"type:jsonb;default:'{}';not null" json:"settings"`
	CreatedBy   string  `gorm:"size:255;not null" json:"created_by"`
	
	// Relationships
	Projects []*Project `gorm:"foreignKey:WorkspaceID;constraint:OnDelete:CASCADE" json:"projects,omitempty"`
	Members  []*WorkspaceMember `gorm:"foreignKey:WorkspaceID;constraint:OnDelete:CASCADE" json:"members,omitempty"`
}

// TableName sets the table name for Workspace
func (Workspace) TableName() string {
	return "workspaces"
}

// Project represents a project within a workspace
type Project struct {
	BaseModel
	WorkspaceID string  `gorm:"size:255;not null;index" json:"workspace_id"`
	Name        string  `gorm:"size:255;not null" json:"name"`
	Description string  `gorm:"type:text" json:"description"`
	Status      string  `gorm:"size:50;not null;default:'active'" json:"status"` // active, archived, deleted
	Settings    JSONMap `gorm:"type:jsonb;default:'{}';not null" json:"settings"`
	CreatedBy   string  `gorm:"size:255;not null" json:"created_by"`
	
	// Relationships
	Workspace     *Workspace     `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
	AirtableBases []*AirtableBase `gorm:"foreignKey:ProjectID;constraint:OnDelete:CASCADE" json:"airtable_bases,omitempty"`
}

// TableName sets the table name for Project
func (Project) TableName() string {
	return "projects"
}

// AirtableBase represents an Airtable base connection
type AirtableBase struct {
	BaseModel
	ProjectID   string     `gorm:"size:255;not null;index" json:"project_id"`
	BaseID      string     `gorm:"size:255;not null" json:"base_id"`
	Name        string     `gorm:"size:255;not null" json:"name"`
	Description string     `gorm:"type:text" json:"description"`
	SyncEnabled bool       `gorm:"default:true" json:"sync_enabled"`
	LastSyncAt  *time.Time `json:"last_sync_at,omitempty"`
	
	// Relationships
	Project *Project `gorm:"foreignKey:ProjectID" json:"project,omitempty"`
}

// TableName sets the table name for AirtableBase
func (AirtableBase) TableName() string {
	return "airtable_bases"
}

// WorkspaceMemberRole represents workspace member roles
type WorkspaceMemberRole string

const (
	WorkspaceRoleOwner  WorkspaceMemberRole = "owner"
	WorkspaceRoleAdmin  WorkspaceMemberRole = "admin"
	WorkspaceRoleMember WorkspaceMemberRole = "member"
	WorkspaceRoleViewer WorkspaceMemberRole = "viewer"
)

// WorkspaceMember represents a user's membership in a workspace
type WorkspaceMember struct {
	WorkspaceID string                `gorm:"size:255;not null;primaryKey" json:"workspace_id"`
	UserID      string                `gorm:"size:255;not null;primaryKey" json:"user_id"`
	Role        WorkspaceMemberRole   `gorm:"size:50;not null" json:"role"`
	JoinedAt    time.Time             `gorm:"default:now()" json:"joined_at"`
	
	// Relationships
	Workspace *Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
}

// TableName sets the table name for WorkspaceMember
func (WorkspaceMember) TableName() string {
	return "workspace_members"
}

// WorkspaceAuditLog represents audit log entries for workspace activities
type WorkspaceAuditLog struct {
	ID           string    `gorm:"primarykey;type:uuid;default:gen_random_uuid()" json:"id"`
	WorkspaceID  string    `gorm:"size:255;not null;index" json:"workspace_id"`
	UserID       string    `gorm:"size:255;not null" json:"user_id"`
	Action       string    `gorm:"size:100;not null" json:"action"`
	ResourceType string    `gorm:"size:50;not null" json:"resource_type"`
	ResourceID   string    `gorm:"size:255" json:"resource_id"`
	Changes      JSONMap   `gorm:"type:jsonb" json:"changes"`
	CreatedAt    time.Time `gorm:"default:now()" json:"created_at"`
	
	// Relationships
	Workspace *Workspace `gorm:"foreignKey:WorkspaceID" json:"workspace,omitempty"`
}

// TableName sets the table name for WorkspaceAuditLog
func (WorkspaceAuditLog) TableName() string {
	return "workspace_audit_logs"
}

// Request and Response Models

// CreateWorkspaceRequest represents a workspace creation request
type CreateWorkspaceRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	Description string  `json:"description"`
	Settings    JSONMap `json:"settings,omitempty"`
}

// UpdateWorkspaceRequest represents a workspace update request
type UpdateWorkspaceRequest struct {
	Name        *string  `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Description *string  `json:"description,omitempty"`
	Settings    *JSONMap `json:"settings,omitempty"`
}

// CreateProjectRequest represents a project creation request
type CreateProjectRequest struct {
	Name        string  `json:"name" validate:"required,min=1,max=255"`
	Description string  `json:"description"`
	Settings    JSONMap `json:"settings,omitempty"`
}

// UpdateProjectRequest represents a project update request
type UpdateProjectRequest struct {
	Name        *string  `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Description *string  `json:"description,omitempty"`
	Status      *string  `json:"status,omitempty" validate:"omitempty,oneof=active archived"`
	Settings    *JSONMap `json:"settings,omitempty"`
}

// CreateAirtableBaseRequest represents an Airtable base creation request
type CreateAirtableBaseRequest struct {
	BaseID      string `json:"base_id" validate:"required"`
	Name        string `json:"name" validate:"required,min=1,max=255"`
	Description string `json:"description"`
	SyncEnabled bool   `json:"sync_enabled"`
}

// UpdateAirtableBaseRequest represents an Airtable base update request
type UpdateAirtableBaseRequest struct {
	Name        *string `json:"name,omitempty" validate:"omitempty,min=1,max=255"`
	Description *string `json:"description,omitempty"`
	SyncEnabled *bool   `json:"sync_enabled,omitempty"`
}

// AddWorkspaceMemberRequest represents a request to add a member to workspace
type AddWorkspaceMemberRequest struct {
	UserID string                `json:"user_id" validate:"required"`
	Role   WorkspaceMemberRole   `json:"role" validate:"required,oneof=owner admin member viewer"`
}

// UpdateWorkspaceMemberRequest represents a request to update member role
type UpdateWorkspaceMemberRequest struct {
	Role WorkspaceMemberRole `json:"role" validate:"required,oneof=owner admin member viewer"`
}

// List Response Models

// WorkspaceListResponse represents a paginated list of workspaces
type WorkspaceListResponse struct {
	Workspaces []*Workspace `json:"workspaces"`
	Total      int64        `json:"total"`
	Page       int          `json:"page"`
	PageSize   int          `json:"page_size"`
	TotalPages int          `json:"total_pages"`
}

// ProjectListResponse represents a paginated list of projects
type ProjectListResponse struct {
	Projects   []*Project `json:"projects"`
	Total      int64      `json:"total"`
	Page       int        `json:"page"`
	PageSize   int        `json:"page_size"`
	TotalPages int        `json:"total_pages"`
}

// AirtableBaseListResponse represents a paginated list of Airtable bases
type AirtableBaseListResponse struct {
	Bases      []*AirtableBase `json:"bases"`
	Total      int64           `json:"total"`
	Page       int             `json:"page"`
	PageSize   int             `json:"page_size"`
	TotalPages int             `json:"total_pages"`
}

// WorkspaceMemberListResponse represents a list of workspace members
type WorkspaceMemberListResponse struct {
	Members    []*WorkspaceMember `json:"members"`
	Total      int64              `json:"total"`
	Page       int                `json:"page"`
	PageSize   int                `json:"page_size"`
	TotalPages int                `json:"total_pages"`
}

// AuditLogListResponse represents a paginated list of audit logs
type AuditLogListResponse struct {
	Logs       []*WorkspaceAuditLog `json:"logs"`
	Total      int64                `json:"total"`
	Page       int                  `json:"page"`
	PageSize   int                  `json:"page_size"`
	TotalPages int                  `json:"total_pages"`
}

// Filter Models

// WorkspaceFilter represents filters for listing workspaces
type WorkspaceFilter struct {
	TenantID       string `query:"tenant_id"`
	Search         string `query:"search"`
	CreatedBy      string `query:"created_by"`
	Page           int    `query:"page"`
	PageSize       int    `query:"page_size"`
	SortBy         string `query:"sort_by"`
	SortOrder      string `query:"sort_order"`
	IncludeDeleted bool   `query:"include_deleted"`
}

// ProjectFilter represents filters for listing projects
type ProjectFilter struct {
	WorkspaceID    string `query:"workspace_id"`
	Status         string `query:"status"`
	Search         string `query:"search"`
	CreatedBy      string `query:"created_by"`
	Page           int    `query:"page"`
	PageSize       int    `query:"page_size"`
	SortBy         string `query:"sort_by"`
	SortOrder      string `query:"sort_order"`
	IncludeDeleted bool   `query:"include_deleted"`
}

// AirtableBaseFilter represents filters for listing Airtable bases
type AirtableBaseFilter struct {
	ProjectID   string `query:"project_id"`
	SyncEnabled *bool  `query:"sync_enabled"`
	Search      string `query:"search"`
	Page        int    `query:"page"`
	PageSize    int    `query:"page_size"`
	SortBy      string `query:"sort_by"`
	SortOrder   string `query:"sort_order"`
}

// AuditLogFilter represents filters for listing audit logs
type AuditLogFilter struct {
	WorkspaceID  string `query:"workspace_id"`
	UserID       string `query:"user_id"`
	Action       string `query:"action"`
	ResourceType string `query:"resource_type"`
	ResourceID   string `query:"resource_id"`
	Page         int    `query:"page"`
	PageSize     int    `query:"page_size"`
	SortBy       string `query:"sort_by"`
	SortOrder    string `query:"sort_order"`
}

// Statistics Models

// WorkspaceStats represents workspace statistics
type WorkspaceStats struct {
	TotalWorkspaces      int64              `json:"total_workspaces"`
	ActiveWorkspaces     int64              `json:"active_workspaces"`
	TotalProjects        int64              `json:"total_projects"`
	ActiveProjects       int64              `json:"active_projects"`
	TotalAirtableBases   int64              `json:"total_airtable_bases"`
	WorkspacesByTenant   map[string]int64   `json:"workspaces_by_tenant"`
	ProjectsByStatus     map[string]int64   `json:"projects_by_status"`
	LastUpdated          time.Time          `json:"last_updated"`
}