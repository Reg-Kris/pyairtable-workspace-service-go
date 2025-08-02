package repositories

import (
	"context"

	"go.uber.org/zap"
	"gorm.io/gorm"

	"github.com/Reg-Kris/pyairtable-workspace-service/internal/models"
)

type workspaceMemberRepository struct {
	db     *gorm.DB
	logger *zap.Logger
}

// NewWorkspaceMemberRepository creates a new workspace member repository
func NewWorkspaceMemberRepository(db *gorm.DB, logger *zap.Logger) WorkspaceMemberRepository {
	return &workspaceMemberRepository{
		db:     db,
		logger: logger,
	}
}

// Add adds a member to a workspace
func (r *workspaceMemberRepository) Add(ctx context.Context, member *models.WorkspaceMember) error {
	// Check if member already exists
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.WorkspaceMember{}).
		Where("workspace_id = ? AND user_id = ?", member.WorkspaceID, member.UserID).
		Count(&count).Error; err != nil {
		r.logger.Error("Failed to check duplicate member", zap.Error(err))
		return err
	}
	
	if count > 0 {
		return ErrDuplicateMember
	}

	if err := r.db.WithContext(ctx).Create(member).Error; err != nil {
		r.logger.Error("Failed to add workspace member", zap.Error(err))
		return err
	}

	return nil
}

// GetByWorkspaceAndUser retrieves a member by workspace and user ID
func (r *workspaceMemberRepository) GetByWorkspaceAndUser(ctx context.Context, workspaceID, userID string) (*models.WorkspaceMember, error) {
	var member models.WorkspaceMember
	if err := r.db.WithContext(ctx).
		Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
		First(&member).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return nil, ErrMemberNotFound
		}
		r.logger.Error("Failed to get workspace member", zap.Error(err))
		return nil, err
	}

	return &member, nil
}

// UpdateRole updates a member's role
func (r *workspaceMemberRepository) UpdateRole(ctx context.Context, workspaceID, userID string, role models.WorkspaceMemberRole) error {
	// Check if trying to remove last owner
	if role != models.WorkspaceRoleOwner {
		isLastOwner, err := r.IsLastOwner(ctx, workspaceID, userID)
		if err != nil {
			return err
		}
		if isLastOwner {
			return ErrLastOwner
		}
	}

	result := r.db.WithContext(ctx).Model(&models.WorkspaceMember{}).
		Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
		Update("role", role)
		
	if result.Error != nil {
		r.logger.Error("Failed to update member role", zap.Error(result.Error))
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrMemberNotFound
	}

	return nil
}

// Remove removes a member from a workspace
func (r *workspaceMemberRepository) Remove(ctx context.Context, workspaceID, userID string) error {
	// Check if member is owner
	member, err := r.GetByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		return err
	}

	if member.Role == models.WorkspaceRoleOwner {
		// Check if last owner
		isLastOwner, err := r.IsLastOwner(ctx, workspaceID, userID)
		if err != nil {
			return err
		}
		if isLastOwner {
			return ErrLastOwner
		}
	}

	result := r.db.WithContext(ctx).
		Where("workspace_id = ? AND user_id = ?", workspaceID, userID).
		Delete(&models.WorkspaceMember{})
		
	if result.Error != nil {
		r.logger.Error("Failed to remove workspace member", zap.Error(result.Error))
		return result.Error
	}

	if result.RowsAffected == 0 {
		return ErrMemberNotFound
	}

	return nil
}

// List retrieves workspace members with pagination
func (r *workspaceMemberRepository) List(ctx context.Context, workspaceID string, page, pageSize int) ([]*models.WorkspaceMember, int64, error) {
	query := r.db.WithContext(ctx).Model(&models.WorkspaceMember{}).
		Where("workspace_id = ?", workspaceID)

	// Count total records
	var total int64
	if err := query.Count(&total).Error; err != nil {
		r.logger.Error("Failed to count workspace members", zap.Error(err))
		return nil, 0, err
	}

	// Apply pagination
	if page < 1 {
		page = 1
	}
	if pageSize < 1 {
		pageSize = 20
	}
	if pageSize > 100 {
		pageSize = 100
	}
	
	offset := (page - 1) * pageSize
	query = query.Offset(offset).Limit(pageSize)

	// Order by joined date
	query = query.Order("joined_at DESC")

	// Fetch members
	var members []*models.WorkspaceMember
	if err := query.Find(&members).Error; err != nil {
		r.logger.Error("Failed to list workspace members", zap.Error(err))
		return nil, 0, err
	}

	return members, total, nil
}

// CountOwners counts the number of owners in a workspace
func (r *workspaceMemberRepository) CountOwners(ctx context.Context, workspaceID string) (int64, error) {
	var count int64
	if err := r.db.WithContext(ctx).Model(&models.WorkspaceMember{}).
		Where("workspace_id = ? AND role = ?", workspaceID, models.WorkspaceRoleOwner).
		Count(&count).Error; err != nil {
		r.logger.Error("Failed to count workspace owners", zap.Error(err))
		return 0, err
	}

	return count, nil
}

// IsLastOwner checks if the user is the last owner of the workspace
func (r *workspaceMemberRepository) IsLastOwner(ctx context.Context, workspaceID, userID string) (bool, error) {
	// First check if user is an owner
	member, err := r.GetByWorkspaceAndUser(ctx, workspaceID, userID)
	if err != nil {
		return false, err
	}

	if member.Role != models.WorkspaceRoleOwner {
		return false, nil
	}

	// Count total owners
	ownerCount, err := r.CountOwners(ctx, workspaceID)
	if err != nil {
		return false, err
	}

	return ownerCount <= 1, nil
}