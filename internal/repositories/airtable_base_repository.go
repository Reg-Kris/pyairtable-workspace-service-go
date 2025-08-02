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

type airtableBaseRepository struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewAirtableBaseRepository creates a new Airtable base repository
func NewAirtableBaseRepository(db *gorm.DB, logger *zap.Logger) AirtableBaseRepository {
	return &airtableBaseRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new Airtable base connection
func (r *airtableBaseRepository) Create(ctx context.Context, base *models.AirtableBase) error {
	// Check if base is already connected to the project
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.AirtableBase{}).
		Where("project_id = ? AND base_id = ? AND deleted_at IS NULL", base.ProjectID, base.BaseID).
		Count(&count).Error; err != nil {
		r.logger.Error("Failed to check duplicate airtable base", zap.Error(err))
		return err
	}
	
	if count > 0 {
		return ErrDuplicateAirtableBase
	}

	if err := r.db.WithContext(ctx).Create(base).Error; err != nil {
		r.logger.Error("Failed to create airtable base", zap.Error(err))
		return err
	}

	return nil
}

// GetByID retrieves an Airtable base by ID
func (r *airtableBaseRepository) GetByID(ctx context.Context, id string) (*models.AirtableBase, error) {
	var base models.AirtableBase
	if err := r.db.WithContext(ctx).
		Preload("Project").
		Preload("Project.Workspace").
		Where("id = ? AND deleted_at IS NULL", id).
		First(&base).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrAirtableBaseNotFound
		}
		r.logger.Error("Failed to get airtable base by ID", zap.Error(err), zap.String("id", id))
		return nil, err
	}

	return &base, nil
}

// GetByProjectAndBaseID retrieves an Airtable base by project ID and base ID
func (r *airtableBaseRepository) GetByProjectAndBaseID(ctx context.Context, projectID, baseID string) (*models.AirtableBase, error) {
	var base models.AirtableBase
	if err := r.db.WithContext(ctx).
		Where("project_id = ? AND base_id = ? AND deleted_at IS NULL", projectID, baseID).
		First(&base).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrAirtableBaseNotFound
		}
		r.logger.Error("Failed to get airtable base by project and base ID", zap.Error(err))
		return nil, err
	}

	return &base, nil
}

// Update updates an Airtable base
func (r *airtableBaseRepository) Update(ctx context.Context, base *models.AirtableBase) error {
	result := r.db.WithContext(ctx).Model(base).Updates(base)
	if result.Error != nil {
		r.logger.Error("Failed to update airtable base", zap.Error(result.Error))
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrAirtableBaseNotFound
	}

	return nil
}

// Delete soft deletes an Airtable base
func (r *airtableBaseRepository) Delete(ctx context.Context, id string) error {
	result := r.db.WithContext(ctx).Where("id = ?", id).Delete(&models.AirtableBase{})
	if result.Error != nil {
		r.logger.Error("Failed to delete airtable base", zap.Error(result.Error))
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrAirtableBaseNotFound
	}

	return nil
}

// List retrieves Airtable bases based on filter
func (r *airtableBaseRepository) List(ctx context.Context, filter *models.AirtableBaseFilter) ([]*models.AirtableBase, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.AirtableBase{})

	// Apply filters
	if filter.ProjectID != "" {
		query = query.Where("project_id = ?", filter.ProjectID)
	}

	if filter.SyncEnabled != nil {
		query = query.Where("sync_enabled = ?", *filter.SyncEnabled)
	}

	if filter.Search != "" {
		search := "%" + strings.ToLower(filter.Search) + "%"
		query = query.Where("LOWER(name) LIKE ? OR LOWER(description) LIKE ? OR LOWER(base_id) LIKE ?", search, search, search)
	}

	// Always exclude soft deleted records
	query = query.Where("deleted_at IS NULL")

	// Count total records
	var total int64
	if err := query.Count(&total).Error; err != nil {
		r.logger.Error("Failed to count airtable bases", zap.Error(err))
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
	query = query.Preload("Project").Preload("Project.Workspace")

	// Fetch bases
	var bases []*models.AirtableBase
	if err := query.Find(&bases).Error; err != nil {
		r.logger.Error("Failed to list airtable bases", zap.Error(err))
		return nil, 0, err
	}

	return bases, total, nil
}

// UpdateSyncTime updates the last sync time for an Airtable base
func (r *airtableBaseRepository) UpdateSyncTime(ctx context.Context, id string, syncTime time.Time) error {
	result := r.db.WithContext(ctx).Model(&models.AirtableBase{}).
		Where("id = ? AND deleted_at IS NULL", id).
		Update("last_sync_at", syncTime)
		
	if result.Error != nil {
		r.logger.Error("Failed to update sync time", zap.Error(result.Error))
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrAirtableBaseNotFound
	}

	return nil
}