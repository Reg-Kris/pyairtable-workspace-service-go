package services

import (
	"context"
	"fmt"

	"go.uber.org/zap"

	"github.com/Reg-Kris/pyairtable-workspace-service/internal/config"
	"github.com/Reg-Kris/pyairtable-workspace-service/internal/models"
	"github.com/Reg-Kris/pyairtable-workspace-service/internal/repositories"
)

type projectService struct {
	repos        *repositories.Repositories
	config       *config.Config
	logger       *zap.Logger
	auditService AuditService
}

// NewProjectService creates a new project service
func NewProjectService(repos *repositories.Repositories, config *config.Config, logger *zap.Logger, auditService AuditService) ProjectService {
	return &projectService{
		repos:        repos,
		config:       config,
		logger:       logger,
		auditService: auditService,
	}
}

// CreateProject creates a new project in a workspace
func (s *projectService) CreateProject(ctx context.Context, workspaceID, userID string, req *models.CreateProjectRequest) (*models.Project, error) {
	// Check workspace access
	workspace, err := s.repos.Workspace.GetByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	// Check user has at least member role in workspace
	member, err := s.repos.Member.GetByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		if err == repositories.ErrMemberNotFound {
			return nil, ErrUnauthorized
		}
		return nil, err
	}

	if !hasRequiredRole(member.Role, models.WorkspaceRoleMember) {
		return nil, ErrUnauthorized
	}

	// TODO: Check project quota for workspace
	const maxProjectsPerWorkspace = 50
	
	projectCount, err := s.repos.Project.CountByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}

	if projectCount >= maxProjectsPerWorkspace {
		return nil, ErrQuotaExceeded
	}

	// Create project
	project := &models.Project{
		WorkspaceID: workspaceID,
		Name:        req.Name,
		Description: req.Description,
		Status:      "active",
		Settings:    req.Settings,
		CreatedBy:   userID,
	}

	if project.Settings == nil {
		project.Settings = make(models.JSONMap)
	}

	if err := s.repos.Project.Create(ctx, project); err != nil {
		return nil, err
	}

	// Set workspace reference
	project.Workspace = workspace

	// Cache the project
	_ = s.repos.Cache.SetProject(ctx, project)

	// Log audit
	_ = s.auditService.LogAction(ctx, workspaceID, userID, "project.created", "project", project.ID, map[string]interface{}{
		"name":         project.Name,
		"description":  project.Description,
		"workspace_id": workspaceID,
	})

	return project, nil
}

// GetProject retrieves a project by ID
func (s *projectService) GetProject(ctx context.Context, projectID, userID string) (*models.Project, error) {
	// Check cache first
	project, err := s.repos.Cache.GetProject(ctx, projectID)
	if err == nil && project != nil {
		// Verify user has access to the workspace
		if err := s.checkProjectAccess(ctx, project, userID, models.WorkspaceRoleViewer); err != nil {
			return nil, err
		}
		return project, nil
	}

	// Get from database
	project, err = s.repos.Project.GetByID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Check access
	if err := s.checkProjectAccess(ctx, project, userID, models.WorkspaceRoleViewer); err != nil {
		return nil, err
	}

	// Cache the project
	_ = s.repos.Cache.SetProject(ctx, project)

	return project, nil
}

// UpdateProject updates a project
func (s *projectService) UpdateProject(ctx context.Context, projectID, userID string, req *models.UpdateProjectRequest) (*models.Project, error) {
	// Get existing project
	project, err := s.repos.Project.GetByID(ctx, projectID)
	if err != nil {
		return nil, err
	}

	// Check access - need at least member role
	if err := s.checkProjectAccess(ctx, project, userID, models.WorkspaceRoleMember); err != nil {
		return nil, err
	}

	// Track changes for audit
	changes := make(map[string]interface{})

	// Update fields
	if req.Name != nil {
		changes["name"] = map[string]interface{}{
			"old": project.Name,
			"new": *req.Name,
		}
		project.Name = *req.Name
	}

	if req.Description != nil {
		changes["description"] = map[string]interface{}{
			"old": project.Description,
			"new": *req.Description,
		}
		project.Description = *req.Description
	}

	if req.Status != nil {
		// Validate status
		validStatuses := []string{"active", "archived"}
		isValid := false
		for _, s := range validStatuses {
			if *req.Status == s {
				isValid = true
				break
			}
		}
		if !isValid {
			return nil, fmt.Errorf("invalid status: %s", *req.Status)
		}

		changes["status"] = map[string]interface{}{
			"old": project.Status,
			"new": *req.Status,
		}
		project.Status = *req.Status
	}

	if req.Settings != nil {
		changes["settings"] = map[string]interface{}{
			"old": project.Settings,
			"new": *req.Settings,
		}
		project.Settings = *req.Settings
	}

	// Update in database
	if err := s.repos.Project.Update(ctx, project); err != nil {
		return nil, err
	}

	// Invalidate cache
	_ = s.repos.Cache.DeleteProject(ctx, projectID)

	// Log audit
	if len(changes) > 0 {
		_ = s.auditService.LogAction(ctx, project.WorkspaceID, userID, "project.updated", "project", projectID, changes)
	}

	return project, nil
}

// DeleteProject deletes a project
func (s *projectService) DeleteProject(ctx context.Context, projectID, userID string) error {
	// Get project
	project, err := s.repos.Project.GetByID(ctx, projectID)
	if err != nil {
		return err
	}

	// Check access - need at least admin role
	if err := s.checkProjectAccess(ctx, project, userID, models.WorkspaceRoleAdmin); err != nil {
		return err
	}

	// Check if project has connected Airtable bases
	baseFilter := &models.AirtableBaseFilter{
		ProjectID: projectID,
		PageSize:  1,
	}
	
	_, baseCount, err := s.repos.AirtableBase.List(ctx, baseFilter)
	if err != nil {
		return err
	}

	if baseCount > 0 {
		return fmt.Errorf("cannot delete project with %d connected Airtable bases", baseCount)
	}

	// Delete project
	if err := s.repos.Project.Delete(ctx, projectID); err != nil {
		return err
	}

	// Invalidate cache
	_ = s.repos.Cache.DeleteProject(ctx, projectID)

	// Log audit
	_ = s.auditService.LogAction(ctx, project.WorkspaceID, userID, "project.deleted", "project", projectID, map[string]interface{}{
		"name": project.Name,
	})

	return nil
}

// ListProjects lists projects based on filter
func (s *projectService) ListProjects(ctx context.Context, filter *models.ProjectFilter, userID string) (*models.ProjectListResponse, error) {
	// If workspace ID is provided, check access
	if filter.WorkspaceID != "" {
		member, err := s.repos.Member.GetByWorkspaceAndUser(ctx, filter.WorkspaceID, userID)
		if err != nil {
			if err == repositories.ErrMemberNotFound {
				return &models.ProjectListResponse{
					Projects:   []*models.Project{},
					Total:      0,
					Page:       1,
					PageSize:   filter.PageSize,
					TotalPages: 0,
				}, nil
			}
			return nil, err
		}

		// Viewers and above can list projects
		if !hasRequiredRole(member.Role, models.WorkspaceRoleViewer) {
			return nil, ErrUnauthorized
		}
	}

	projects, total, err := s.repos.Project.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	// If no workspace ID filter, filter by user's accessible workspaces
	if filter.WorkspaceID == "" {
		// TODO: Implement filtering by user's workspaces
		// For now, we'll return all projects (simplified)
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

	return &models.ProjectListResponse{
		Projects:   projects,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// checkProjectAccess checks if user has required access to a project
func (s *projectService) checkProjectAccess(ctx context.Context, project *models.Project, userID string, requiredRole models.WorkspaceMemberRole) error {
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