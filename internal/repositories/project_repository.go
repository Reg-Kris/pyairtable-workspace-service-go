package repositories

import (
	"context"
	"fmt"
	"strings"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/Reg-Kris/pyairtable-workspace-service/internal/models"
)

type projectRepository struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewProjectRepository creates a new project repository
func NewProjectRepository(db *gorm.DB, logger *zap.Logger) ProjectRepository {
	return &projectRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new project
func (r *projectRepository) Create(ctx context.Context, project *models.Project) error {
	// Check if project with same name exists in workspace
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Project{}).
		Where("workspace_id = ? AND name = ? AND deleted_at IS NULL", project.WorkspaceID, project.Name).
		Count(&count).Error; err != nil {
		r.logger.Error("Failed to check duplicate project", zap.Error(err))
		return err
	}
	
	if count > 0 {
		return ErrDuplicateProject
	}

	if err := r.db.WithContext(ctx).Create(project).Error; err != nil {
		r.logger.Error("Failed to create project", zap.Error(err))
		return err
	}

	return nil
}

// GetByID retrieves a project by ID
func (r *projectRepository) GetByID(ctx context.Context, id string) (*models.Project, error) {
	var project models.Project
	if err := r.db.WithContext(ctx).
		Preload("Workspace").
		Where("id = ? AND deleted_at IS NULL", id).
		First(&project).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrProjectNotFound
		}
		r.logger.Error("Failed to get project by ID", zap.Error(err), zap.String("id", id))
		return nil, err
	}

	return &project, nil
}

// GetByWorkspaceAndName retrieves a project by workspace ID and name
func (r *projectRepository) GetByWorkspaceAndName(ctx context.Context, workspaceID, name string) (*models.Project, error) {
	var project models.Project
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND name = ? AND deleted_at IS NULL", workspaceID, name).
		First(&project).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrProjectNotFound
		}
		r.logger.Error("Failed to get project by workspace and name", zap.Error(err))
		return nil, err
	}

	return &project, nil
}

// Update updates a project
func (r *projectRepository) Update(ctx context.Context, project *models.Project) error {
	// Check if another project with same name exists
	if project.Name != "" {
		var count int64
		if err := r.db.WithContext(ctx).Model(&models.Project{}).
			Where("workspace_id = ? AND name = ? AND id != ? AND deleted_at IS NULL", 
				project.WorkspaceID, project.Name, project.ID).
			Count(&count).Error; err != nil {
			r.logger.Error("Failed to check duplicate project", zap.Error(err))
			return err
		}
		
		if count > 0 {
			return ErrDuplicateProject
		}
	}

	result := r.db.WithContext(ctx).Model(project).Updates(project)
	if result.Error != nil {
		r.logger.Error("Failed to update project", zap.Error(result.Error))
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrProjectNotFound
	}

	return nil
}

// Delete soft deletes a project
func (r *projectRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.Project{})
	if result.Error != nil {
		r.logger.Error("Failed to delete project", zap.Error(result.Error))
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrProjectNotFound
	}

	return nil
}

// List retrieves projects based on filter
func (r *projectRepository) List(ctx context.Context, filter *models.ProjectFilter) ([]*models.Project, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.Project{})

	// Apply filters
	if filter.WorkspaceID != "" {
		query = query.Where("workspace_id = ?", filter.WorkspaceID)
	}

	if filter.Status != "" {
		query = query.Where("status = ?", filter.Status)
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
		r.logger.Error("Failed to count projects", zap.Error(err))
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

	// Preload associations
	query = query.Preload("Workspace")

	// Fetch projects
	var projects []*models.Project
	if err := query.Find(&projects).Error; err != nil {
		r.logger.Error("Failed to list projects", zap.Error(err))
		return nil, 0, err
	}

	return projects, total, nil
}

// CountByWorkspace counts projects in a workspace
func (r *projectRepository) CountByWorkspace(ctx context.Context, workspaceID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.Project{}).
		Where("workspace_id = ? AND deleted_at IS NULL", workspaceID).
		Count(&count).Error; err != nil {
		r.logger.Error("Failed to count projects by workspace", zap.Error(err))
		return 0, err
	}

	return count, nil
}