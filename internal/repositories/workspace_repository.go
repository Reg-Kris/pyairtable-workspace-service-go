package repositories

import (
	"context"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/Reg-Kris/pyairtable-workspace-service/internal/models"
)

type workspaceRepository struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewWorkspaceRepository creates a new workspace repository
func NewWorkspaceRepository(db *gorm.DB, logger *zap.Logger) WorkspaceRepository {
	return &workspaceRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new workspace
func (r *workspaceRepository) Create(ctx context.Context, workspace *models.Workspace) error {
	// Check if workspace with same name exists for tenant
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Workspace{}).
		Where("tenant_id = ? AND name = ? AND deleted_at IS NULL", workspace.TenantID, workspace.Name).
		Count(&count).Error; err != nil {
		r.logger.Error("Failed to check duplicate workspace", zap.Error(err))
		return err
	}
	
	if count > 0 {
		return ErrDuplicateWorkspace
	}

	if err := r.db.WithContext(ctx).Create(workspace).Error; err != nil {
		r.logger.Error("Failed to create workspace", zap.Error(err))
		return err
	}

	return nil
}

// GetByID retrieves a workspace by ID
func (r *workspaceRepository) GetByID(ctx context.Context, id string) (*models.Workspace, error) {
	var workspace models.Workspace
	if err := r.db.WithContext(ctx).
		Where("id = ? AND deleted_at IS NULL", id).
		First(&workspace).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrWorkspaceNotFound
		}
		r.logger.Error("Failed to get workspace by ID", zap.Error(err), zap.String("id", id))
		return nil, err
	}

	return &workspace, nil
}

// GetByTenantAndName retrieves a workspace by tenant ID and name
func (r *workspaceRepository) GetByTenantAndName(ctx context.Context, tenantID, name string) (*models.Workspace, error) {
	var workspace models.Workspace
	if err := r.db.WithContext(ctx).
		Where("tenant_id = ? AND name = ? AND deleted_at IS NULL", tenantID, name).
		First(&workspace).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrWorkspaceNotFound
		}
		r.logger.Error("Failed to get workspace by tenant and name", zap.Error(err))
		return nil, err
	}

	return &workspace, nil
}

// Update updates a workspace
func (r *workspaceRepository) Update(ctx context.Context, workspace *models.Workspace) error {
	// Check if another workspace with same name exists
	if workspace.Name != "" {
		var count int64
		if err := r.db.WithContext(ctx).Model(&models.Workspace{}).
			Where("tenant_id = ? AND name = ? AND id != ? AND deleted_at IS NULL", 
				workspace.TenantID, workspace.Name, workspace.ID).
			Count(&count).Error; err != nil {
			r.logger.Error("Failed to check duplicate workspace", zap.Error(err))
			return err
		}
		
		if count > 0 {
			return ErrDuplicateWorkspace
		}
	}

	result := r.db.WithContext(ctx).Model(workspace).Updates(workspace)
	if result.Error != nil {
		r.logger.Error("Failed to update workspace", zap.Error(result.Error))
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrWorkspaceNotFound
	}

	return nil
}

// Delete soft deletes a workspace
func (r *workspaceRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.Workspace{})
	if result.Error != nil {
		r.logger.Error("Failed to delete workspace", zap.Error(result.Error))
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrWorkspaceNotFound
	}

	return nil
}

// List retrieves workspaces based on filter
func (r *workspaceRepository) List(ctx context.Context, filter *models.WorkspaceFilter) ([]*models.Workspace, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.Workspace{})

	// Apply filters
	if filter.TenantID != "" {
		query = query.Where("tenant_id = ?", filter.TenantID)
	}

	if filter.CreatedBy != "" {
		query = query.Where("created_by = ?", filter.CreatedBy)
	}

	if filter.Search != "" {
		search := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", search, search)
	}

	if !filter.IncludeDeleted {
		query = query.Where("deleted_at IS NULL")
	}

	// Count total records
	var total int64
	if err := query.Count(&total).Error; err != nil {
		r.logger.Error("Failed to count workspaces", zap.Error(err))
		return nil, 0, err
	}

	// Apply sorting
	sortBy := "created_at"
	if filter.SortBy != "" {
		sortBy = filter.SortBy
	}
	
	sortOrder := "DESC"
	if filter.SortOrder != "" && strings.ToUpper(filter.SortOrder) == "ASC" {
		sortOrder = "ASC"
	}
	
	query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))

	// Apply pagination
	page := filter.Page
	if page < 1 {
		page = 1
	}
	
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	
	offset := (page - 1) * pageSize
	query = query.Offset(offset).Limit(pageSize)

	// Fetch workspaces
	var workspaces []*models.Workspace
	if err := query.Find(&workspaces).Error; err != nil {
		r.logger.Error("Failed to list workspaces", zap.Error(err))
		return nil, 0, err
	}

	return workspaces, total, nil
}

// GetStats retrieves workspace statistics for a tenant
func (r *workspaceRepository) GetStats(ctx context.Context, tenantID string) (*models.WorkspaceStats, error) {
	stats := &models.WorkspaceStats{
		WorkspacesByTenant: make(map[string]int64),
		ProjectsByStatus:   make(map[string]int64),
	}

	// Total workspaces
	if err := r.db.WithContext(ctx).Model(&models.Workspace{}).
		Where("tenant_id = ? AND deleted_at IS NULL", tenantID).
		Count(&stats.TotalWorkspaces).Error; err != nil {
		r.logger.Error("Failed to count total workspaces", zap.Error(err))
		return nil, err
	}

	// Active workspaces (same as total for now, could add status field later)
	stats.ActiveWorkspaces = stats.TotalWorkspaces

	// Total projects
	if err := r.db.WithContext(ctx).
		Table("projects").
		Joins("JOIN workspaces ON projects.workspace_id = workspaces.id").
		Where("workspaces.tenant_id = ? AND projects.deleted_at IS NULL", tenantID).
		Count(&stats.TotalProjects).Error; err != nil {
		r.logger.Error("Failed to count total projects", zap.Error(err))
		return nil, err
	}

	// Active projects
	if err := r.db.WithContext(ctx).
		Table("projects").
		Joins("JOIN workspaces ON projects.workspace_id = workspaces.id").
		Where("workspaces.tenant_id = ? AND projects.status = 'active' AND projects.deleted_at IS NULL", tenantID).
		Count(&stats.ActiveProjects).Error; err != nil {
		r.logger.Error("Failed to count active projects", zap.Error(err))
		return nil, err
	}

	// Total Airtable bases
	if err := r.db.WithContext(ctx).
		Table("airtable_bases").
		Joins("JOIN projects ON airtable_bases.project_id = projects.id").
		Joins("JOIN workspaces ON projects.workspace_id = workspaces.id").
		Where("workspaces.tenant_id = ? AND airtable_bases.deleted_at IS NULL", tenantID).
		Count(&stats.TotalAirtableBases).Error; err != nil {
		r.logger.Error("Failed to count total airtable bases", zap.Error(err))
		return nil, err
	}

	// Projects by status
	type statusCount struct {
		Status string
		Count  int64
	}
	var statusCounts []statusCount
	if err := r.db.WithContext(ctx).
		Table("projects").
		Select("projects.status, COUNT(*) as count").
		Joins("JOIN workspaces ON projects.workspace_id = workspaces.id").
		Where("workspaces.tenant_id = ? AND projects.deleted_at IS NULL", tenantID).
		Group("projects.status").
		Scan(&statusCounts).Error; err != nil {
		r.logger.Error("Failed to count projects by status", zap.Error(err))
		return nil, err
	}

	for _, sc := range statusCounts {
		stats.ProjectsByStatus[sc.Status] = sc.Count
	}

	stats.LastUpdated = time.Now()

	return stats, nil
}