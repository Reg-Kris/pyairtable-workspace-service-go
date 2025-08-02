package services

import (
	"context"
	"time"

	"go.uber.org/zap"

	"github.com/Reg-Kris/pyairtable-workspace-service/internal/config"
	"github.com/Reg-Kris/pyairtable-workspace-service/internal/models"
	"github.com/Reg-Kris/pyairtable-workspace-service/internal/repositories"
)

type airtableBaseService struct {
	repos        *repositories.Repositories
	config       *config.Config
	logger       *zap.Logger
	auditService AuditService
}

// NewAirtableBaseService creates a new Airtable base service
func NewAirtableBaseService(repos *repositories.Repositories, config *config.Config, logger *zap.Logger, auditService AuditService) AirtableBaseService {
	return &airtableBaseService{
		repos:        repos,
		config:       config,
		logger:       logger,
		auditService: auditService,
	}
}

// ConnectBase connects an Airtable base to a project
func (s *airtableBaseService) ConnectBase(ctx context.Context, projectID, userID string, req *models.CreateAirtableBaseRequest) (*models.AirtableBase, error) {
	// Get project and check access
	project, err := s.repos.Project.GetByID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Check user has at least member role in workspace
	if err := s.checkProjectAccess(ctx, project, userID, models.WorkspaceRoleMember); err != nil {
		return nil, err
	}

	// TODO: Validate base exists in Airtable via Airtable Gateway
	// For now, we'll trust the base ID

	// Create Airtable base connection
	base := &models.AirtableBase{
		ProjectID:   projectID,
		BaseID:      req.BaseID,
		Name:        req.Name,
		Description: req.Description,
		SyncEnabled: req.SyncEnabled,
	}

	if err := s.repos.AirtableBase.Create(ctx, base); err != nil {
		return nil, err
	}

	// Set project reference
	base.Project = project

	// Log audit
	_ = s.auditService.LogAction(ctx, project.WorkspaceID, userID, "airtable_base.connected", "airtable_base", base.ID, map[string]interface{}{
		"base_id":      req.BaseID,
		"name":         req.Name,
		"project_id":   projectID,
		"sync_enabled": req.SyncEnabled,
	})

	return base, nil
}

// GetBase retrieves an Airtable base by ID
func (s *airtableBaseService) GetBase(ctx context.Context, baseID, userID string) (*models.AirtableBase, error) {
	// Get base from database
	base, err := s.repos.AirtableBase.GetByID(ctx, baseID)
	if err != nil {
		return nil, err
	}

	// Check access via project
	if base.Project == nil {
		// Load project if not preloaded
		project, err := s.repos.Project.GetByID(ctx, base.ProjectID)
		if err != nil {
			return nil, err
		}
		base.Project = project
	}

	if err := s.checkProjectAccess(ctx, base.Project, userID, models.WorkspaceRoleViewer); err != nil {
		return nil, err
	}

	return base, nil
}

// UpdateBase updates an Airtable base connection
func (s *airtableBaseService) UpdateBase(ctx context.Context, baseID, userID string, req *models.UpdateAirtableBaseRequest) (*models.AirtableBase, error) {
	// Get existing base
	base, err := s.repos.AirtableBase.GetByID(ctx, baseID)
	if err != nil {
		return nil, err
	}

	// Load project if needed
	if base.Project == nil {
		project, err := s.repos.Project.GetByID(ctx, base.ProjectID)
		if err != nil {
			return nil, err
		}
		base.Project = project
	}

	// Check access - need at least member role
	if err := s.checkProjectAccess(ctx, base.Project, userID, models.WorkspaceRoleMember); err != nil {
		return nil, err
	}

	// Track changes for audit
	changes := make(map[string]interface{})

	// Update fields
	if req.Name != nil {
		changes["name"] = map[string]interface{}{
			"old": base.Name,
			"new": *req.Name,
		}
		base.Name = *req.Name
	}

	if req.Description != nil {
		changes["description"] = map[string]interface{}{
			"old": base.Description,
			"new": *req.Description,
		}
		base.Description = *req.Description
	}

	if req.SyncEnabled != nil {
		changes["sync_enabled"] = map[string]interface{}{
			"old": base.SyncEnabled,
			"new": *req.SyncEnabled,
		}
		base.SyncEnabled = *req.SyncEnabled
	}

	// Update in database
	if err := s.repos.AirtableBase.Update(ctx, base); err != nil {
		return nil, err
	}

	// Log audit
	if len(changes) > 0 {
		_ = s.auditService.LogAction(ctx, base.Project.WorkspaceID, userID, "airtable_base.updated", "airtable_base", baseID, changes)
	}

	return base, nil
}

// DisconnectBase disconnects an Airtable base from a project
func (s *airtableBaseService) DisconnectBase(ctx context.Context, baseID, userID string) error {
	// Get base
	base, err := s.repos.AirtableBase.GetByID(ctx, baseID)
	if err != nil {
		return err
	}

	// Load project if needed
	if base.Project == nil {
		project, err := s.repos.Project.GetByID(ctx, base.ProjectID)
		if err != nil {
			return err
		}
		base.Project = project
	}

	// Check access - need at least admin role
	if err := s.checkProjectAccess(ctx, base.Project, userID, models.WorkspaceRoleAdmin); err != nil {
		return err
	}

	// Delete base connection
	if err := s.repos.AirtableBase.Delete(ctx, baseID); err != nil {
		return err
	}

	// Log audit
	_ = s.auditService.LogAction(ctx, base.Project.WorkspaceID, userID, "airtable_base.disconnected", "airtable_base", baseID, map[string]interface{}{
		"base_id": base.BaseID,
		"name":    base.Name,
	})

	return nil
}

// ListBases lists Airtable bases based on filter
func (s *airtableBaseService) ListBases(ctx context.Context, filter *models.AirtableBaseFilter, userID string) (*models.AirtableBaseListResponse, error) {
	// If project ID is provided, check access
	if filter.ProjectID != "" {
		project, err := s.repos.Project.GetByID(ctx, filter.ProjectID)
		if err != nil {
			return nil, err
		}

		if err := s.checkProjectAccess(ctx, project, userID, models.WorkspaceRoleViewer); err != nil {
			return &models.AirtableBaseListResponse{
				Bases:      []*models.AirtableBase{},
				Total:      0,
				Page:       1,
				PageSize:   filter.PageSize,
				TotalPages: 0,
			}, nil
		}
	}

	bases, total, err := s.repos.AirtableBase.List(ctx, filter)
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
		pageSize = 20
	}
	
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	return &models.AirtableBaseListResponse{
		Bases:      bases,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// UpdateSyncStatus updates the sync status of an Airtable base
func (s *airtableBaseService) UpdateSyncStatus(ctx context.Context, baseID string) error {
	// This would typically be called by a sync service
	// For now, just update the last sync time
	now := time.Now()
	if err := s.repos.AirtableBase.UpdateSyncTime(ctx, baseID, now); err != nil {
		return err
	}

	s.logger.Info("Updated Airtable base sync status",
		zap.String("base_id", baseID),
		zap.Time("sync_time", now))

	return nil
}

// checkProjectAccess checks if user has required access to a project
func (s *airtableBaseService) checkProjectAccess(ctx context.Context, project *models.Project, userID string, requiredRole models.WorkspaceMemberRole) error {
	member, err := s.repos.Member.GetByWorkspaceAndUser(ctx, project.WorkspaceID, userID)
	if err != nil {
		if err == repositories.ErrMemberNotFound {
			return ErrUnauthorized
		}
		return err
	}

	if !hasRequiredRole(member.Role, requiredRole) {
		return ErrUnauthorized
	}

	return nil
}