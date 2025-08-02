package repositories

import (
	"context"
	"encoding/json"
	"time"

	"github.com/redis/go-redis/v9"
	"go.uber.org/zap"

	"github.com/Reg-Kris/pyairtable-workspace-service/internal/models"
)

const (
	workspaceCachePrefix = "workspace:"
	projectCachePrefix   = "project:"
	userWorkspacePrefix  = "user:workspaces:"
	cacheTTL             = 5 * time.Minute
)

type cacheRepository struct {
	redis  *redis.Client
	logger *zap.Logger
}

// NewCacheRepository creates a new cache repository
func NewCacheRepository(redis *redis.Client, logger *zap.Logger) CacheRepository {
	return &cacheRepository{
		redis:  redis,
		logger: logger,
	}
}

// SetWorkspace caches a workspace
func (r *cacheRepository) SetWorkspace(ctx context.Context, workspace *models.Workspace) error {
	key := workspaceCachePrefix + workspace.ID
	
	data, err := json.Marshal(workspace)
	if err != nil {
		r.logger.Error("Failed to marshal workspace", zap.Error(err))
		return err
	}

	if err := r.redis.Set(ctx, key, data, cacheTTL).Err(); err != nil {
		r.logger.Error("Failed to cache workspace", zap.Error(err))
		return err
	}

	return nil
}

// GetWorkspace retrieves a workspace from cache
func (r *cacheRepository) GetWorkspace(ctx context.Context, id string) (*models.Workspace, error) {
	key := workspaceCachePrefix + id
	
	data, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		r.logger.Error("Failed to get workspace from cache", zap.Error(err))
		return nil, err
	}

	var workspace models.Workspace
	if err := json.Unmarshal([]byte(data), &workspace); err != nil {
		r.logger.Error("Failed to unmarshal workspace", zap.Error(err))
		return nil, err
	}

	return &workspace, nil
}

// DeleteWorkspace removes a workspace from cache
func (r *cacheRepository) DeleteWorkspace(ctx context.Context, id string) error {
	key := workspaceCachePrefix + id
	
	if err := r.redis.Del(ctx, key).Err(); err != nil {
		r.logger.Error("Failed to delete workspace from cache", zap.Error(err))
		return err
	}

	return nil
}

// SetProject caches a project
func (r *cacheRepository) SetProject(ctx context.Context, project *models.Project) error {
	key := projectCachePrefix + project.ID
	
	data, err := json.Marshal(project)
	if err != nil {
		r.logger.Error("Failed to marshal project", zap.Error(err))
		return err
	}

	if err := r.redis.Set(ctx, key, data, cacheTTL).Err(); err != nil {
		r.logger.Error("Failed to cache project", zap.Error(err))
		return err
	}

	return nil
}

// GetProject retrieves a project from cache
func (r *cacheRepository) GetProject(ctx context.Context, id string) (*models.Project, error) {
	key := projectCachePrefix + id
	
	data, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		r.logger.Error("Failed to get project from cache", zap.Error(err))
		return nil, err
	}

	var project models.Project
	if err := json.Unmarshal([]byte(data), &project); err != nil {
		r.logger.Error("Failed to unmarshal project", zap.Error(err))
		return nil, err
	}

	return &project, nil
}

// DeleteProject removes a project from cache
func (r *cacheRepository) DeleteProject(ctx context.Context, id string) error {
	key := projectCachePrefix + id
	
	if err := r.redis.Del(ctx, key).Err(); err != nil {
		r.logger.Error("Failed to delete project from cache", zap.Error(err))
		return err
	}

	return nil
}

// InvalidateWorkspaceCache invalidates all cache entries related to a workspace
func (r *cacheRepository) InvalidateWorkspaceCache(ctx context.Context, workspaceID string) error {
	// Delete workspace cache
	if err := r.DeleteWorkspace(ctx, workspaceID); err != nil {
		return err
	}

	// Find and delete all projects in this workspace
	// This is a simplified approach - in production, you might want to maintain
	// a list of project IDs per workspace
	pattern := projectCachePrefix + "*"
	iter := r.redis.Scan(ctx, 0, pattern, 100).Iterator()
	
	for iter.Next(ctx) {
		key := iter.Val()
		// Check if this project belongs to the workspace
		data, err := r.redis.Get(ctx, key).Result()
		if err != nil {
			continue
		}
		
		var project models.Project
		if err := json.Unmarshal([]byte(data), &project); err != nil {
			continue
		}
		
		if project.WorkspaceID == workspaceID {
			r.redis.Del(ctx, key)
		}
	}
	
	if err := iter.Err(); err != nil {
		r.logger.Error("Failed to scan Redis keys", zap.Error(err))
		return err
	}

	return nil
}

// SetUserWorkspaces caches the list of workspace IDs for a user
func (r *cacheRepository) SetUserWorkspaces(ctx context.Context, userID string, workspaceIDs []string) error {
	key := userWorkspacePrefix + userID
	
	data, err := json.Marshal(workspaceIDs)
	if err != nil {
		r.logger.Error("Failed to marshal workspace IDs", zap.Error(err))
		return err
	}

	if err := r.redis.Set(ctx, key, data, cacheTTL).Err(); err != nil {
		r.logger.Error("Failed to cache user workspaces", zap.Error(err))
		return err
	}

	return nil
}

// GetUserWorkspaces retrieves the list of workspace IDs for a user from cache
func (r *cacheRepository) GetUserWorkspaces(ctx context.Context, userID string) ([]string, error) {
	key := userWorkspacePrefix + userID
	
	data, err := r.redis.Get(ctx, key).Result()
	if err != nil {
		if err == redis.Nil {
			return nil, nil // Cache miss
		}
		r.logger.Error("Failed to get user workspaces from cache", zap.Error(err))
		return nil, err
	}

	var workspaceIDs []string
	if err := json.Unmarshal([]byte(data), &workspaceIDs); err != nil {
		r.logger.Error("Failed to unmarshal workspace IDs", zap.Error(err))
		return nil, err
	}

	return workspaceIDs, nil
}

// Additional helper methods for cache warming and invalidation

// WarmWorkspaceCache warms the cache with workspace data
func (r *cacheRepository) WarmWorkspaceCache(ctx context.Context, workspaces []*models.Workspace) error {
	pipe := r.redis.Pipeline()
	
	for _, workspace := range workspaces {
		key := workspaceCachePrefix + workspace.ID
		data, err := json.Marshal(workspace)
		if err != nil {
			r.logger.Error("Failed to marshal workspace for warming", zap.Error(err))
			continue
		}
		pipe.Set(ctx, key, data, cacheTTL)
	}
	
	if _, err := pipe.Exec(ctx); err != nil {
		r.logger.Error("Failed to warm workspace cache", zap.Error(err))
		return err
	}
	
	return nil
}

// InvalidateUserCache invalidates all cache entries for a user
func (r *cacheRepository) InvalidateUserCache(ctx context.Context, userID string) error {
	key := userWorkspacePrefix + userID
	
	if err := r.redis.Del(ctx, key).Err(); err != nil {
		r.logger.Error("Failed to invalidate user cache", zap.Error(err))
		return err
	}
	
	return nil
}

// ClearAllCache clears all workspace-related cache entries
func (r *cacheRepository) ClearAllCache(ctx context.Context) error {
	patterns := []string{
		workspaceCachePrefix + "*",
		projectCachePrefix + "*",
		userWorkspacePrefix + "*",
	}
	
	for _, pattern := range patterns {
		iter := r.redis.Scan(ctx, 0, pattern, 100).Iterator()
		for iter.Next(ctx) {
			if err := r.redis.Del(ctx, iter.Val()).Err(); err != nil {
				r.logger.Error("Failed to delete cache key", 
					zap.String("key", iter.Val()),
					zap.Error(err))
			}
		}
		if err := iter.Err(); err != nil {
			r.logger.Error("Failed to scan Redis keys", 
				zap.String("pattern", pattern),
				zap.Error(err))
			return err
		}
	}
	
	return nil
}