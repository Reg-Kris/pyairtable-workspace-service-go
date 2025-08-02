package handlers

import (
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"go.uber.org/zap"

	"github.com/Reg-Kris/pyairtable-workspace-service/internal/models"
	"github.com/Reg-Kris/pyairtable-workspace-service/internal/services"
)

// Handlers aggregates all handler functions
type Handlers struct {
	services *services.Services
	logger   *zap.Logger
}

// New creates a new Handlers instance
func New(services *services.Services, logger *zap.Logger) *Handlers {
	return &Handlers{
		services: services,
		logger:   logger,
	}
}

// Health handles health check requests
func (h *Handlers) Health(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"status":    "healthy",
		"service":   "workspace-service",
		"timestamp": time.Now(),
	})
}

// getUserID extracts user ID from context
func (h *Handlers) getUserID(c *fiber.Ctx) string {
	userID := c.Locals("user_id")
	if userID == nil {
		return ""
	}
	return userID.(string)
}

// getTenantID extracts tenant ID from context
func (h *Handlers) getTenantID(c *fiber.Ctx) string {
	tenantID := c.Locals("tenant_id")
	if tenantID == nil {
		return ""
	}
	return tenantID.(string)
}

// handleError returns appropriate error response
func (h *Handlers) handleError(c *fiber.Ctx, err error) error {
	switch err {
	case services.ErrWorkspaceNotFound:
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Workspace not found",
		})
	case services.ErrProjectNotFound:
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Project not found",
		})
	case services.ErrAirtableBaseNotFound:
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"error": "Airtable base not found",
		})
	case services.ErrUnauthorized:
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Unauthorized",
		})
	case services.ErrQuotaExceeded:
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
			"error": "Quota exceeded",
		})
	case services.ErrInvalidInput:
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid input",
		})
	default:
		h.logger.Error("Unhandled error", zap.Error(err))
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Internal server error",
		})
	}
}

// Workspace Handlers

// CreateWorkspace creates a new workspace
func (h *Handlers) CreateWorkspace(c *fiber.Ctx) error {
	tenantID := h.getTenantID(c)
	userID := h.getUserID(c)

	if tenantID == "" || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication context",
		})
	}

	var req models.CreateWorkspaceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	workspace, err := h.services.Workspace.CreateWorkspace(c.Context(), tenantID, userID, &req)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(workspace)
}

// GetWorkspace retrieves a workspace by ID
func (h *Handlers) GetWorkspace(c *fiber.Ctx) error {
	workspaceID := c.Params("id")
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	workspace, err := h.services.Workspace.GetWorkspace(c.Context(), workspaceID, userID)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(workspace)
}

// UpdateWorkspace updates a workspace
func (h *Handlers) UpdateWorkspace(c *fiber.Ctx) error {
	workspaceID := c.Params("id")
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	var req models.UpdateWorkspaceRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	workspace, err := h.services.Workspace.UpdateWorkspace(c.Context(), workspaceID, userID, &req)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(workspace)
}

// DeleteWorkspace deletes a workspace
func (h *Handlers) DeleteWorkspace(c *fiber.Ctx) error {
	workspaceID := c.Params("id")
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	if err := h.services.Workspace.DeleteWorkspace(c.Context(), workspaceID, userID); err != nil {
		return h.handleError(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ListWorkspaces lists workspaces
func (h *Handlers) ListWorkspaces(c *fiber.Ctx) error {
	tenantID := h.getTenantID(c)
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	filter := &models.WorkspaceFilter{
		TenantID: tenantID,
	}

	// Parse query parameters
	if err := c.QueryParser(filter); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid query parameters",
		})
	}

	response, err := h.services.Workspace.ListWorkspaces(c.Context(), filter, userID)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(response)
}

// GetWorkspaceStats retrieves workspace statistics
func (h *Handlers) GetWorkspaceStats(c *fiber.Ctx) error {
	tenantID := h.getTenantID(c)
	userID := h.getUserID(c)

	if tenantID == "" || userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication context",
		})
	}

	stats, err := h.services.Workspace.GetWorkspaceStats(c.Context(), tenantID, userID)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(stats)
}

// Project Handlers

// CreateProject creates a new project
func (h *Handlers) CreateProject(c *fiber.Ctx) error {
	workspaceID := c.Params("workspace_id")
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	var req models.CreateProjectRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	project, err := h.services.Project.CreateProject(c.Context(), workspaceID, userID, &req)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(project)
}

// GetProject retrieves a project by ID
func (h *Handlers) GetProject(c *fiber.Ctx) error {
	projectID := c.Params("id")
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	project, err := h.services.Project.GetProject(c.Context(), projectID, userID)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(project)
}

// UpdateProject updates a project
func (h *Handlers) UpdateProject(c *fiber.Ctx) error {
	projectID := c.Params("id")
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	var req models.UpdateProjectRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	project, err := h.services.Project.UpdateProject(c.Context(), projectID, userID, &req)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(project)
}

// DeleteProject deletes a project
func (h *Handlers) DeleteProject(c *fiber.Ctx) error {
	projectID := c.Params("id")
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	if err := h.services.Project.DeleteProject(c.Context(), projectID, userID); err != nil {
		return h.handleError(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ListProjects lists projects
func (h *Handlers) ListProjects(c *fiber.Ctx) error {
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	filter := &models.ProjectFilter{}

	// Parse query parameters
	if err := c.QueryParser(filter); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid query parameters",
		})
	}

	response, err := h.services.Project.ListProjects(c.Context(), filter, userID)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(response)
}

// Airtable Base Handlers

// ConnectAirtableBase connects an Airtable base to a project
func (h *Handlers) ConnectAirtableBase(c *fiber.Ctx) error {
	projectID := c.Params("project_id")
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	var req models.CreateAirtableBaseRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	base, err := h.services.AirtableBase.ConnectBase(c.Context(), projectID, userID, &req)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(base)
}

// GetAirtableBase retrieves an Airtable base by ID
func (h *Handlers) GetAirtableBase(c *fiber.Ctx) error {
	baseID := c.Params("id")
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	base, err := h.services.AirtableBase.GetBase(c.Context(), baseID, userID)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(base)
}

// UpdateAirtableBase updates an Airtable base
func (h *Handlers) UpdateAirtableBase(c *fiber.Ctx) error {
	baseID := c.Params("id")
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	var req models.UpdateAirtableBaseRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	base, err := h.services.AirtableBase.UpdateBase(c.Context(), baseID, userID, &req)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(base)
}

// DisconnectAirtableBase disconnects an Airtable base
func (h *Handlers) DisconnectAirtableBase(c *fiber.Ctx) error {
	baseID := c.Params("id")
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	if err := h.services.AirtableBase.DisconnectBase(c.Context(), baseID, userID); err != nil {
		return h.handleError(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ListAirtableBases lists Airtable bases
func (h *Handlers) ListAirtableBases(c *fiber.Ctx) error {
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	filter := &models.AirtableBaseFilter{}

	// Parse query parameters
	if err := c.QueryParser(filter); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid query parameters",
		})
	}

	response, err := h.services.AirtableBase.ListBases(c.Context(), filter, userID)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(response)
}

// Member Handlers

// AddWorkspaceMember adds a member to a workspace
func (h *Handlers) AddWorkspaceMember(c *fiber.Ctx) error {
	workspaceID := c.Params("workspace_id")
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	var req models.AddWorkspaceMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	member, err := h.services.Member.AddMember(c.Context(), workspaceID, userID, &req)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(member)
}

// UpdateWorkspaceMemberRole updates a member's role
func (h *Handlers) UpdateWorkspaceMemberRole(c *fiber.Ctx) error {
	workspaceID := c.Params("workspace_id")
	memberUserID := c.Params("user_id")
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	var req models.UpdateWorkspaceMemberRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid request body",
		})
	}

	if err := h.services.Member.UpdateMemberRole(c.Context(), workspaceID, memberUserID, userID, &req); err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"message": "Member role updated successfully",
	})
}

// RemoveWorkspaceMember removes a member from a workspace
func (h *Handlers) RemoveWorkspaceMember(c *fiber.Ctx) error {
	workspaceID := c.Params("workspace_id")
	memberUserID := c.Params("user_id")
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	if err := h.services.Member.RemoveMember(c.Context(), workspaceID, memberUserID, userID); err != nil {
		return h.handleError(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// ListWorkspaceMembers lists workspace members
func (h *Handlers) ListWorkspaceMembers(c *fiber.Ctx) error {
	workspaceID := c.Params("workspace_id")
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	page, _ := strconv.Atoi(c.Query("page", "1"))
	pageSize, _ := strconv.Atoi(c.Query("page_size", "20"))

	response, err := h.services.Member.ListMembers(c.Context(), workspaceID, userID, page, pageSize)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(response)
}

// GetUserWorkspaces retrieves all workspaces for a user
func (h *Handlers) GetUserWorkspaces(c *fiber.Ctx) error {
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	workspaces, err := h.services.Member.GetUserWorkspaces(c.Context(), userID)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"workspaces": workspaces,
		"total":      len(workspaces),
	})
}

// Audit Log Handlers

// GetAuditLogs retrieves audit logs
func (h *Handlers) GetAuditLogs(c *fiber.Ctx) error {
	userID := h.getUserID(c)

	if userID == "" {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"error": "Missing authentication",
		})
	}

	filter := &models.AuditLogFilter{}

	// Parse query parameters
	if err := c.QueryParser(filter); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error": "Invalid query parameters",
		})
	}

	response, err := h.services.Audit.GetAuditLogs(c.Context(), filter, userID)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(response)
}