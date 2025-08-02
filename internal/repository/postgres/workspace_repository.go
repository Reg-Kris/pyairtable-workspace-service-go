package postgres

import (
	"context"
	"fmt"
	"strings"
	"time"
	
	"github.com/Reg-Kris/pyairtable-workspace-service/internal/models"
	"github.com/Reg-Kris/pyairtable-workspace-service/internal/repository"
	"gorm.io/gorm"
	"go.uber.org/zap"
)

type workspaceRepository struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewWorkspaceRepository creates a new workspace repository
func NewWorkspaceRepository(db *gorm.DB, logger *zap.Logger) repository.WorkspaceRepository {
	return &workspaceRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new workspace
func (r *workspaceRepository) Create(ctx context.Context, workspace *models.Workspace) error {
	if err := r.db.WithContext(ctx).Create(workspace).Error; err != nil {
		r.logger.Error("Failed to create workspace", zap.Error(err))
		return err
	}
	
	r.logger.Info("Workspace created", zap.String("id", workspace.ID), zap.String("tenant_id", workspace.TenantID))
	return nil
}

// FindByID finds a workspace by ID
func (r *workspaceRepository) FindByID(ctx context.Context, id string) (*models.Workspace, error) {
	var workspace models.Workspace
	
	if err := r.db.WithContext(ctx).
		Preload("Projects").
		Preload("Members").
		First(&workspace, "id = ?", id).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, fmt.Errorf("workspace not found")
		}
		r.logger.Error("Failed to find workspace", zap.Error(err), zap.String("id", id))
		return nil, err
	}
	
	return &workspace, nil
}

// FindByTenantID finds workspaces by tenant ID with filtering
func (r *workspaceRepository) FindByTenantID(ctx context.Context, tenantID string, filter *models.WorkspaceFilter) ([]*models.Workspace, int64, error) {
	var workspaces []*models.Workspace
	var total int64
	
	query := r.db.WithContext(ctx).Model(&models.Workspace{}).Where("tenant_id = ?", tenantID)
	
	// Apply filters
	query = r.applyWorkspaceFilters(query, filter)
	
	// Count total
	if err := query.Count(&total).Error; err != nil {
		r.logger.Error("Failed to count workspaces", zap.Error(err))
		return nil, 0, err
	}
	
	// Apply pagination and sorting
	query = r.applyPaginationAndSorting(query, filter)
	
	if err := query.Find(&workspaces).Error; err != nil {
		r.logger.Error("Failed to find workspaces by tenant", zap.Error(err))
		return nil, 0, err
	}
	
	return workspaces, total, nil
}

// Update updates a workspace
func (r *workspaceRepository) Update(ctx context.Context, workspace *models.Workspace) error {
	if err := r.db.WithContext(ctx).Save(workspace).Error; err != nil {
		r.logger.Error("Failed to update workspace", zap.Error(err))
		return err
	}
	
	r.logger.Info("Workspace updated", zap.String("id", workspace.ID))
	return nil
}

// SoftDelete soft deletes a workspace
func (r *workspaceRepository) SoftDelete(ctx context.Context, id string) error {
	if err := r.db.WithContext(ctx).Delete(&models.Workspace{}, "id = ?", id).Error; err != nil {
		r.logger.Error("Failed to soft delete workspace", zap.Error(err))
		return err
	}
	
	r.logger.Info("Workspace soft deleted", zap.String("id", id))
	return nil
}

// List lists workspaces with filtering and pagination
func (r *workspaceRepository) List(ctx context.Context, filter *models.WorkspaceFilter) ([]*models.Workspace, int64, error) {
	var workspaces []*models.Workspace
	var total int64
	
	query := r.db.WithContext(ctx).Model(&models.Workspace{})
	
	// Apply filters
	query = r.applyWorkspaceFilters(query, filter)
	
	// Count total
	if err := query.Count(&total).Error; err != nil {
		r.logger.Error("Failed to count workspaces", zap.Error(err))
		return nil, 0, err
	}
	
	// Apply pagination and sorting
	query = r.applyPaginationAndSorting(query, filter)
	
	if err := query.Find(&workspaces).Error; err != nil {
		r.logger.Error("Failed to list workspaces", zap.Error(err))
		return nil, 0, err
	}
	
	return workspaces, total, nil
}

// GetStats gets workspace statistics
func (r *workspaceRepository) GetStats(ctx context.Context, tenantID string) (*models.WorkspaceStats, error) {
	stats := &models.WorkspaceStats{
		LastUpdated: time.Now(),
	}
	
	query := r.db.WithContext(ctx).Model(&models.Workspace{})
	if tenantID != "" {
		query = query.Where("tenant_id = ?", tenantID)
	}
	
	// Total workspaces
	if err := query.Count(&stats.TotalWorkspaces).Error; err != nil {
		return nil, err
	}
	
	// Active workspaces (non-deleted)
	stats.ActiveWorkspaces = stats.TotalWorkspaces // Since we're using soft deletes
	
	// Total projects
	projectQuery := r.db.WithContext(ctx).Model(&models.Project{})
	if tenantID != "" {
		projectQuery = projectQuery.Joins("JOIN workspaces ON projects.workspace_id = workspaces.id").
			Where("workspaces.tenant_id = ?", tenantID)
	}
	if err := projectQuery.Count(&stats.TotalProjects).Error; err != nil {
		return nil, err
	}
	
	// Active projects
	activeProjectQuery := projectQuery.Where("projects.status = ?", "active")
	if err := activeProjectQuery.Count(&stats.ActiveProjects).Error; err != nil {
		return nil, err
	}
	
	// Total Airtable bases
	baseQuery := r.db.WithContext(ctx).Model(&models.AirtableBase{})
	if tenantID != "" {
		baseQuery = baseQuery.Joins("JOIN projects ON airtable_bases.project_id = projects.id").
			Joins("JOIN workspaces ON projects.workspace_id = workspaces.id").
			Where("workspaces.tenant_id = ?", tenantID)
	}
	if err := baseQuery.Count(&stats.TotalAirtableBases).Error; err != nil {
		return nil, err
	}
	
	// Projects by status
	stats.ProjectsByStatus = make(map[string]int64)
	rows, err := r.db.WithContext(ctx).Raw(`
		SELECT p.status, COUNT(*) 
		FROM projects p 
		JOIN workspaces w ON p.workspace_id = w.id 
		WHERE ($1 = '' OR w.tenant_id = $1) AND p.deleted_at IS NULL
		GROUP BY p.status
	`, tenantID).Rows()
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	
	for rows.Next() {
		var status string
		var count int64
		if err := rows.Scan(&status, &count); err != nil {
			continue
		}
		stats.ProjectsByStatus[status] = count
	}
	
	// Workspaces by tenant (only if global stats)
	if tenantID == "" {
		stats.WorkspacesByTenant = make(map[string]int64)
		rows, err := r.db.WithContext(ctx).Raw(`
			SELECT tenant_id, COUNT(*) 
			FROM workspaces 
			WHERE deleted_at IS NULL
			GROUP BY tenant_id
		`).Rows()
		if err != nil {
			return nil, err
		}
		defer rows.Close()
		
		for rows.Next() {
			var tid string
			var count int64
			if err := rows.Scan(&tid, &count); err != nil {
				continue
			}
			stats.WorkspacesByTenant[tid] = count
		}
	}
	
	return stats, nil
}

// CountByTenant counts workspaces by tenant
func (r *workspaceRepository) CountByTenant(ctx context.Context, tenantID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Workspace{}).
		Where("tenant_id = ?", tenantID).Count(&count).Error; err != nil {
		return 0, err
	}
	return count, nil
}

// Helper methods

func (r *workspaceRepository) applyWorkspaceFilters(query *gorm.DB, filter *models.WorkspaceFilter) *gorm.DB {
	if filter == nil {
		return query
	}
	
	if filter.TenantID != "" {
		query = query.Where("tenant_id = ?", filter.TenantID)
	}
	
	if filter.CreatedBy != "" {
		query = query.Where("created_by = ?", filter.CreatedBy)
	}
	
	if filter.Search != "" {
		searchTerm := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ?", searchTerm, searchTerm)
	}
	
	if !filter.IncludeDeleted {
		query = query.Where("deleted_at IS NULL")
	}
	
	return query
}

func (r *workspaceRepository) applyPaginationAndSorting(query *gorm.DB, filter *models.WorkspaceFilter) *gorm.DB {
	if filter == nil {
		return query.Order("created_at DESC")
	}
	
	// Sorting
	sortBy := filter.SortBy
	if sortBy == "" {
		sortBy = "created_at"
	}
	
	sortOrder := strings.ToUpper(filter.SortOrder)
	if sortOrder != "ASC" && sortOrder != "DESC" {
		sortOrder = "DESC"
	}
	
	query = query.Order(fmt.Sprintf("%s %s", sortBy, sortOrder))
	
	// Pagination
	if filter.Page > 0 && filter.PageSize > 0 {
		offset := (filter.Page - 1) * filter.PageSize
		query = query.Offset(offset).Limit(filter.PageSize)
	}
	
	return query
}