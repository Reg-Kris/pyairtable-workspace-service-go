package services

import (
	"context"

	"go.uber.org/zap"

	"github.com/Reg-Kris/pyairtable-workspace-service/internal/models"
	"github.com/Reg-Kris/pyairtable-workspace-service/internal/repositories"
)

type auditService struct {
	repos  *repositories.Repositories
	logger *zap.Logger
}

// NewAuditService creates a new audit service
func NewAuditService(repos *repositories.Repositories, logger *zap.Logger) AuditService {
	return &auditService{
		repos:  repos,
		logger: logger,
	}
}

// LogAction logs an action to the audit log
func (s *auditService) LogAction(ctx context.Context, workspaceID, userID, action, resourceType, resourceID string, changes map[string]interface{}) error {
	log := &models.WorkspaceAuditLog{
		WorkspaceID:  workspaceID,
		UserID:       userID,
		Action:       action,
		ResourceType: resourceType,
		ResourceID:   resourceID,
		Changes:      changes,
	}

	if err := s.repos.AuditLog.Create(ctx, log); err != nil {
		// Log error but don't fail the operation
		s.logger.Error("Failed to create audit log",
			zap.Error(err),
			zap.String("workspace_id", workspaceID),
			zap.String("action", action))
		return nil // Don't propagate audit log errors
	}

	return nil
}

// GetAuditLogs retrieves audit logs based on filter
func (s *auditService) GetAuditLogs(ctx context.Context, filter *models.AuditLogFilter, userID string) (*models.AuditLogListResponse, error) {
	// Check if user has access to the workspace
	if filter.WorkspaceID != "" {
		member, err := s.repos.Member.GetByWorkspaceAndUser(ctx, filter.WorkspaceID, userID)
		if err != nil {
			if err == repositories.ErrMemberNotFound {
				return &models.AuditLogListResponse{
					Logs:       []*models.WorkspaceAuditLog{},
					Total:      0,
					Page:       1,
					PageSize:   filter.PageSize,
					TotalPages: 0,
				}, nil
			}
			return nil, err
		}

		// Only admins and owners can view audit logs
		if !hasRequiredRole(member.Role, models.WorkspaceRoleAdmin) {
			return nil, ErrUnauthorized
		}
	}

	logs, total, err := s.repos.AuditLog.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Calculate pagination
	page := filter.Page
	if page < 1 {
		page = 1
	}
	
	pageSize := filter.PageSize
	if pageSize < 1 {
		pageSize = 50
	}
	
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	return &models.AuditLogListResponse{
		Logs:       logs,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// CleanupOldLogs deletes audit logs older than specified days
func (s *auditService) CleanupOldLogs(ctx context.Context, days int) error {
	if days < 30 {
		days = 30 // Minimum retention period
	}

	if err := s.repos.AuditLog.DeleteOlderThan(ctx, days); err != nil {
		s.logger.Error("Failed to cleanup old audit logs", zap.Error(err))
		return err
	}

	s.logger.Info("Cleaned up old audit logs", zap.Int("days", days))
	return nil
}