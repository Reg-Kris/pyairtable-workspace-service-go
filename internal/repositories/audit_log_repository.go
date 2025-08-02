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

type auditLogRepository struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewAuditLogRepository creates a new audit log repository
func NewAuditLogRepository(db *gorm.DB, logger *zap.Logger) AuditLogRepository {
	return &auditLogRepository{
		db:     db,
		logger: logger,
	}
}

// Create creates a new audit log entry
func (r *auditLogRepository) Create(ctx context.Context, log *models.WorkspaceAuditLog) error {
	if err := r.db.WithContext(ctx).Create(log).Error; err != nil {
		r.logger.Error("Failed to create audit log", zap.Error(err))
		return err
	}

	return nil
}

// List retrieves audit logs based on filter
func (r *auditLogRepository) List(ctx context.Context, filter *models.AuditLogFilter) ([]*models.WorkspaceAuditLog, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.WorkspaceAuditLog{})

	// Apply filters
	if filter.WorkspaceID != "" {
		query = query.Where("workspace_id = ?", filter.WorkspaceID)
	}

	if filter.UserID != "" {
		query = query.Where("user_id = ?", filter.UserID)
	}

	if filter.Action != "" {
		query = query.Where("action = ?", filter.Action)
	}

	if filter.ResourceType != "" {
		query = query.Where("resource_type = ?", filter.ResourceType)
	}

	if filter.ResourceID != "" {
		query = query.Where("resource_id = ?", filter.ResourceID)
	}

	// Count total records
	var total int64
	if err := query.Count(&total).Error; err != nil {
		r.logger.Error("Failed to count audit logs", zap.Error(err))
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
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}
	
	offset := (page - 1) * pageSize
	query = query.Offset(offset).Limit(pageSize)

	// Fetch logs
	var logs []*models.WorkspaceAuditLog
	if err := query.Find(&logs).Error; err != nil {
		r.logger.Error("Failed to list audit logs", zap.Error(err))
		return nil, 0, err
	}

	return logs, total, nil
}

// DeleteOlderThan deletes audit logs older than specified days
func (r *auditLogRepository) DeleteOlderThan(ctx context.Context, days int) error {
	cutoffDate := time.Now().AddDate(0, 0, -days)
	
	result := r.db.WithContext(ctx).
		Where("created_at < ?", cutoffDate).
		Delete(&models.WorkspaceAuditLog{})
		
	if result.Error != nil {
		r.logger.Error("Failed to delete old audit logs", zap.Error(result.Error))
		return result.Error
	}

	r.logger.Info("Deleted old audit logs", 
		zap.Int64("count", result.RowsAffected),
		zap.Time("before", cutoffDate))

	return nil
}