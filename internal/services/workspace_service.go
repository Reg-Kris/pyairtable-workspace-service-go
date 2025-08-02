package services

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/Reg-Kris/pyairtable-workspace-service/internal/config"
	"github.com/Reg-Kris/pyairtable-workspace-service/internal/models"
	"github.com/Reg-Kris/pyairtable-workspace-service/internal/repositories"
)

type workspaceService struct {
	repos        *repositories.Repositories
	config       *config.Config
	logger       *zap.Logger
	auditService AuditService
}

// NewWorkspaceService creates a new workspace service
func NewWorkspaceService(repos *repositories.Repositories, config *config.Config, logger *zap.Logger, auditService AuditService) WorkspaceService {
	return &workspaceService{
		repos:        repos,
		config:       config,
		logger:       logger,
		auditService: auditService,
	}
}

// CreateWorkspace creates a new workspace
func (s *workspaceService) CreateWorkspace(ctx context.Context, tenantID, userID string, req *models.CreateWorkspaceRequest) (*models.Workspace, error) {
	// TODO: Check tenant quota via Tenant Service
	// For now, we'll use a hardcoded limit
	const maxWorkspacesPerTenant = 10
	
	// Check current workspace count
	filter := &models.WorkspaceFilter{
		TenantID: tenantID,
		PageSize: 1,
	}
	
	_, count, err := s.repos.Workspace.List(ctx, filter)
	if err != nil {
		s.logger.Error("Failed to count workspaces", zap.Error(err))
		return nil, err
	}
	
	if count >= maxWorkspacesPerTenant {
		return nil, ErrQuotaExceeded
	}

	// Create workspace
	workspace := &models.Workspace{
		TenantID:    tenantID,
		Name:        req.Name,
		Description: req.Description,
		Settings:    req.Settings,
		CreatedBy:   userID,
	}

	if workspace.Settings == nil {
		workspace.Settings = make(models.JSONMap)
	}

	// Begin transaction
	tx := s.repos.BeginTx(ctx)
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
			panic(r)
		}
	}()

	// Create workspace in transaction
	if err := s.repos.Workspace.Create(ctx, workspace); err != nil {
		tx.Rollback()
		return nil, err
	}

	// Add creator as owner
	member := &models.WorkspaceMember{
		WorkspaceID: workspace.ID,
		UserID:      userID,
		Role:        models.WorkspaceRoleOwner,
	}

	if err := s.repos.Member.Add(ctx, member); err != nil {
		tx.Rollback()
		return nil, err
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		s.logger.Error("Failed to commit transaction", zap.Error(err))
		return nil, err
	}

	// Cache the workspace
	_ = s.repos.Cache.SetWorkspace(ctx, workspace)

	// Log audit
	_ = s.auditService.LogAction(ctx, workspace.ID, userID, "workspace.created", "workspace", workspace.ID, map[string]interface{}{
		"name":        workspace.Name,
		"description": workspace.Description,
		"tenant_id":   workspace.TenantID,
	})

	return workspace, nil
}

// GetWorkspace retrieves a workspace by ID
func (s *workspaceService) GetWorkspace(ctx context.Context, workspaceID, userID string) (*models.Workspace, error) {
	// Check cache first
	workspace, err := s.repos.Cache.GetWorkspace(ctx, workspaceID)
	if err == nil && workspace != nil {
		// Check access
		if err := s.CheckUserAccess(ctx, workspaceID, userID, models.WorkspaceRoleViewer); err != nil {
			return nil, err
		}
		return workspace, nil
	}

	// Get from database
	workspace, err = s.repos.Workspace.GetByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	// Check access
	if err := s.CheckUserAccess(ctx, workspaceID, userID, models.WorkspaceRoleViewer); err != nil {
		return nil, err
	}

	// Cache the workspace
	_ = s.repos.Cache.SetWorkspace(ctx, workspace)

	return workspace, nil
}

// UpdateWorkspace updates a workspace
func (s *workspaceService) UpdateWorkspace(ctx context.Context, workspaceID, userID string, req *models.UpdateWorkspaceRequest) (*models.Workspace, error) {
	// Check access
	if err := s.CheckUserAccess(ctx, workspaceID, userID, models.WorkspaceRoleAdmin); err != nil {
		return nil, err
	}

	// Get existing workspace
	workspace, err := s.repos.Workspace.GetByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	// Track changes for audit
	changes := make(map[string]interface{})

	// Update fields
	if req.Name != nil {
		changes["name"] = map[string]interface{}{
			"old": workspace.Name,
			"new": *req.Name,
		}
		workspace.Name = *req.Name
	}

	if req.Description != nil {
		changes["description"] = map[string]interface{}{
			"old": workspace.Description,
			"new": *req.Description,
		}
		workspace.Description = *req.Description
	}

	if req.Settings != nil {
		changes["settings"] = map[string]interface{}{
			"old": workspace.Settings,
			"new": *req.Settings,
		}
		workspace.Settings = *req.Settings
	}

	// Update in database
	if err := s.repos.Workspace.Update(ctx, workspace); err != nil {
		return nil, err
	}

	// Invalidate cache
	_ = s.repos.Cache.DeleteWorkspace(ctx, workspaceID)

	// Log audit
	if len(changes) > 0 {
		_ = s.auditService.LogAction(ctx, workspaceID, userID, "workspace.updated", "workspace", workspaceID, changes)
	}

	return workspace, nil
}

// DeleteWorkspace deletes a workspace
func (s *workspaceService) DeleteWorkspace(ctx context.Context, workspaceID, userID string) error {
	// Check access - only owners can delete
	if err := s.CheckUserAccess(ctx, workspaceID, userID, models.WorkspaceRoleOwner); err != nil {
		return err
	}

	// Check if workspace has active projects
	projectCount, err := s.repos.Project.CountByWorkspace(ctx, workspaceID)
	if err != nil {
		return err
	}

	if projectCount > 0 {
		return fmt.Errorf("cannot delete workspace with %d active projects", projectCount)
	}

	// Delete workspace
	if err := s.repos.Workspace.Delete(ctx, workspaceID); err != nil {
		return err
	}

	// Invalidate cache
	_ = s.repos.Cache.InvalidateWorkspaceCache(ctx, workspaceID)

	// Log audit
	_ = s.auditService.LogAction(ctx, workspaceID, userID, "workspace.deleted", "workspace", workspaceID, nil)

	return nil
}

// ListWorkspaces lists workspaces accessible to the user
func (s *workspaceService) ListWorkspaces(ctx context.Context, filter *models.WorkspaceFilter, userID string) (*models.WorkspaceListResponse, error) {
	// Get user's workspace IDs from cache
	workspaceIDs, err := s.repos.Cache.GetUserWorkspaces(ctx, userID)
	if err != nil || workspaceIDs == nil {
		// Get from database - this would typically query workspace_members table
		// For now, we'll return all workspaces in the tenant (simplified)
		// In production, implement proper member-based filtering
	}

	workspaces, total, err := s.repos.Workspace.List(ctx, filter)
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

	return &models.WorkspaceListResponse{
		Workspaces: workspaces,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetWorkspaceStats retrieves workspace statistics for a tenant
func (s *workspaceService) GetWorkspaceStats(ctx context.Context, tenantID, userID string) (*models.WorkspaceStats, error) {
	// TODO: Check if user has access to tenant stats
	// For now, we'll allow any authenticated user from the tenant
	
	stats, err := s.repos.Workspace.GetStats(ctx, tenantID)
	if err != nil {
		return nil, err
	}

	return stats, nil
}

// CheckUserAccess checks if a user has the required role in a workspace
func (s *workspaceService) CheckUserAccess(ctx context.Context, workspaceID, userID string, requiredRole models.WorkspaceMemberRole) error {
	member, err := s.repos.Member.GetByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		if err == repositories.ErrMemberNotFound {
			return ErrUnauthorized
		}
		return err
	}

	// Check role hierarchy
	if !hasRequiredRole(member.Role, requiredRole) {
		return ErrUnauthorized
	}

	return nil
}

// hasRequiredRole checks if the user's role meets the requirement
func hasRequiredRole(userRole, requiredRole models.WorkspaceMemberRole) bool {
	roleHierarchy := map[models.WorkspaceMemberRole]int{
		models.WorkspaceRoleViewer: 1,
		models.WorkspaceRoleMember: 2,
		models.WorkspaceRoleAdmin:  3,
		models.WorkspaceRoleOwner:  4,
	}

	userLevel, ok1 := roleHierarchy[userRole]
	requiredLevel, ok2 := roleHierarchy[requiredRole]

	if !ok1 || !ok2 {
		return false
	}

	return userLevel >= requiredLevel
}