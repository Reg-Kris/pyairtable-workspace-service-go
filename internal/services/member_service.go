package services

import (
	"context"

	"go.uber.org/zap"

	"github.com/Reg-Kris/pyairtable-workspace-service/internal/config"
	"github.com/Reg-Kris/pyairtable-workspace-service/internal/models"
	"github.com/Reg-Kris/pyairtable-workspace-service/internal/repositories"
)

type memberService struct {
	repos        *repositories.Repositories
	config       *config.Config
	logger       *zap.Logger
	auditService AuditService
}

// NewMemberService creates a new member service
func NewMemberService(repos *repositories.Repositories, config *config.Config, logger *zap.Logger, auditService AuditService) MemberService {
	return &memberService{
		repos:        repos,
		config:       config,
		logger:       logger,
		auditService: auditService,
	}
}

// AddMember adds a member to a workspace
func (s *memberService) AddMember(ctx context.Context, workspaceID, userID string, req *models.AddWorkspaceMemberRequest) (*models.WorkspaceMember, error) {
	// Check if requester has admin access
	requesterMember, err := s.repos.Member.GetByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		if err == repositories.ErrMemberNotFound {
			return nil, ErrUnauthorized
		}
		return nil, err
	}

	// Only admins and owners can add members
	if !hasRequiredRole(requesterMember.Role, models.WorkspaceRoleAdmin) {
		return nil, ErrUnauthorized
	}

	// Validate role assignment rules
	// - Only owners can add other owners
	if req.Role == models.WorkspaceRoleOwner && requesterMember.Role != models.WorkspaceRoleOwner {
		return nil, ErrUnauthorized
	}

	// TODO: Verify target user exists via User Service
	// For now, we'll trust the user ID

	// Create member
	member := &models.WorkspaceMember{
		WorkspaceID: workspaceID,
		UserID:      req.UserID,
		Role:        req.Role,
	}

	if err := s.repos.Member.Add(ctx, member); err != nil {
		return nil, err
	}

	// Invalidate user's workspace cache
	_ = s.repos.Cache.InvalidateUserCache(ctx, req.UserID)

	// Log audit
	_ = s.auditService.LogAction(ctx, workspaceID, userID, "member.added", "workspace_member", req.UserID, map[string]interface{}{
		"user_id": req.UserID,
		"role":    req.Role,
	})

	return member, nil
}

// UpdateMemberRole updates a member's role in a workspace
func (s *memberService) UpdateMemberRole(ctx context.Context, workspaceID, memberUserID, userID string, req *models.UpdateWorkspaceMemberRequest) error {
	// Check if requester has admin access
	requesterMember, err := s.repos.Member.GetByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		if err == repositories.ErrMemberNotFound {
			return ErrUnauthorized
		}
		return err
	}

	// Only admins and owners can update roles
	if !hasRequiredRole(requesterMember.Role, models.WorkspaceRoleAdmin) {
		return ErrUnauthorized
	}

	// Get target member
	targetMember, err := s.repos.Member.GetByWorkspaceAndUser(ctx, workspaceID, memberUserID)
	if err != nil {
		return err
	}

	// Validate role change rules
	// - Only owners can change owner roles
	if targetMember.Role == models.WorkspaceRoleOwner && requesterMember.Role != models.WorkspaceRoleOwner {
		return ErrUnauthorized
	}

	// - Only owners can promote to owner
	if req.Role == models.WorkspaceRoleOwner && requesterMember.Role != models.WorkspaceRoleOwner {
		return ErrUnauthorized
	}

	// - Cannot demote yourself if you're the last owner
	if userID == memberUserID && targetMember.Role == models.WorkspaceRoleOwner && req.Role != models.WorkspaceRoleOwner {
		isLastOwner, err := s.repos.Member.IsLastOwner(ctx, workspaceID, userID)
		if err != nil {
			return err
		}
		if isLastOwner {
			return repositories.ErrLastOwner
		}
	}

	oldRole := targetMember.Role

	// Update role
	if err := s.repos.Member.UpdateRole(ctx, workspaceID, memberUserID, req.Role); err != nil {
		return err
	}

	// Invalidate user's workspace cache
	_ = s.repos.Cache.InvalidateUserCache(ctx, memberUserID)

	// Log audit
	_ = s.auditService.LogAction(ctx, workspaceID, userID, "member.role_updated", "workspace_member", memberUserID, map[string]interface{}{
		"user_id":  memberUserID,
		"old_role": oldRole,
		"new_role": req.Role,
	})

	return nil
}

// RemoveMember removes a member from a workspace
func (s *memberService) RemoveMember(ctx context.Context, workspaceID, memberUserID, userID string) error {
	// Check if requester has admin access
	requesterMember, err := s.repos.Member.GetByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		if err == repositories.ErrMemberNotFound {
			return ErrUnauthorized
		}
		return err
	}

	// Get target member
	targetMember, err := s.repos.Member.GetByWorkspaceAndUser(ctx, workspaceID, memberUserID)
	if err != nil {
		return err
	}

	// Validate removal rules
	// - Members can remove themselves
	// - Admins can remove members and viewers
	// - Owners can remove anyone
	if userID != memberUserID {
		// Not removing self, check permissions
		if !hasRequiredRole(requesterMember.Role, models.WorkspaceRoleAdmin) {
			return ErrUnauthorized
		}

		// Admins cannot remove owners
		if targetMember.Role == models.WorkspaceRoleOwner && requesterMember.Role != models.WorkspaceRoleOwner {
			return ErrUnauthorized
		}
	}

	// Remove member
	if err := s.repos.Member.Remove(ctx, workspaceID, memberUserID); err != nil {
		return err
	}

	// Invalidate user's workspace cache
	_ = s.repos.Cache.InvalidateUserCache(ctx, memberUserID)

	// Log audit
	_ = s.auditService.LogAction(ctx, workspaceID, userID, "member.removed", "workspace_member", memberUserID, map[string]interface{}{
		"user_id": memberUserID,
		"role":    targetMember.Role,
	})

	return nil
}

// ListMembers lists members of a workspace
func (s *memberService) ListMembers(ctx context.Context, workspaceID, userID string, page, pageSize int) (*models.WorkspaceMemberListResponse, error) {
	// Check if user has access to workspace
	member, err := s.repos.Member.GetByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		if err == repositories.ErrMemberNotFound {
			return &models.WorkspaceMemberListResponse{
				Members:    []*models.WorkspaceMember{},
				Total:      0,
				Page:       page,
				PageSize:   pageSize,
				TotalPages: 0,
			}, nil
		}
		return nil, err
	}

	// All members can view member list
	if !hasRequiredRole(member.Role, models.WorkspaceRoleViewer) {
		return nil, ErrUnauthorized
	}

	members, total, err := s.repos.Member.List(ctx, workspaceID, page, pageSize)
	if err != nil {
		return nil, err
	}

	// Calculate pagination
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	
	totalPages := int(total) / pageSize
	if int(total)%pageSize > 0 {
		totalPages++
	}

	return &models.WorkspaceMemberListResponse{
		Members:    members,
		Total:      total,
		Page:       page,
		PageSize:   pageSize,
		TotalPages: totalPages,
	}, nil
}

// GetUserWorkspaces retrieves all workspaces a user is a member of
func (s *memberService) GetUserWorkspaces(ctx context.Context, userID string) ([]*models.Workspace, error) {
	// Check cache first
	workspaceIDs, err := s.repos.Cache.GetUserWorkspaces(ctx, userID)
	if err == nil && workspaceIDs != nil {
		// Load workspaces by IDs
		workspaces := make([]*models.Workspace, 0, len(workspaceIDs))
		for _, id := range workspaceIDs {
			workspace, err := s.repos.Workspace.GetByID(ctx, id)
			if err == nil {
				workspaces = append(workspaces, workspace)
			}
		}
		return workspaces, nil
	}

	// Query all workspaces where user is a member
	// This is a simplified implementation - in production, you'd have a more efficient query
	filter := &models.WorkspaceFilter{
		PageSize: 100, // Get up to 100 workspaces
	}

	allWorkspaces, _, err := s.repos.Workspace.List(ctx, filter)
	if err != nil {
		return nil, err
	}

	// Filter workspaces where user is a member
	userWorkspaces := make([]*models.Workspace, 0)
	workspaceIDs = make([]string, 0)

	for _, workspace := range allWorkspaces {
		member, err := s.repos.Member.GetByWorkspaceAndUser(ctx, workspace.ID, userID)
		if err == nil && member != nil {
			userWorkspaces = append(userWorkspaces, workspace)
			workspaceIDs = append(workspaceIDs, workspace.ID)
		}
	}

	// Cache the result
	if len(workspaceIDs) > 0 {
		_ = s.repos.Cache.SetUserWorkspaces(ctx, userID, workspaceIDs)
	}

	return userWorkspaces, nil
}