package productflow

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/trademind-ai/trademind/backend/internal/pkg/adminperm"
	"github.com/trademind-ai/trademind/backend/internal/pkg/response"
)

type Handler struct{ Svc *Service }

func (h *Handler) tenant(c *gin.Context) (int64, bool) {
	tenantID, err := adminperm.TenantIDFromGin(c)
	if err != nil {
		response.Fail(c, http.StatusForbidden, response.CodeForbidden, "tenant context unavailable")
		return 0, false
	}
	return tenantID, true
}
func (h *Handler) write(c *gin.Context) bool {
	if h == nil || h.Svc == nil || h.Svc.DB == nil {
		response.Fail(c, 500, response.CodeInternalError, "product flow unavailable")
		return false
	}
	if !adminperm.CanWriteProduct(c, h.Svc.DB) {
		response.Fail(c, 403, response.CodeForbidden, "当前账号为只读权限，无法执行此操作")
		return false
	}
	return true
}
func parseID(c *gin.Context) (uuid.UUID, bool) {
	id, err := uuid.Parse(strings.TrimSpace(c.Param("id")))
	if err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid id")
		return uuid.Nil, false
	}
	return id, true
}
func pageQuery(c *gin.Context) ListQuery {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	size, _ := strconv.Atoi(c.DefaultQuery("pageSize", "20"))
	q := ListQuery{Page: page, PageSize: size, Status: strings.TrimSpace(c.Query("status")), Platform: strings.TrimSpace(c.Query("platform")), Keyword: strings.TrimSpace(c.Query("keyword")), SortBy: strings.TrimSpace(c.Query("sortBy")), SortOrder: strings.TrimSpace(c.Query("sortOrder"))}
	if v := strings.TrimSpace(c.Query("catalogProductId")); v != "" {
		if id, e := uuid.Parse(v); e == nil {
			q.CatalogID = &id
		}
	}
	return q
}

func (h *Handler) AnalyzeCandidate(c *gin.Context) {
	if !h.write(c) {
		return
	}
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body AnalyzeCandidateBody
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
			return
		}
	}
	out, err := h.Svc.AnalyzeCandidate(c.Request.Context(), tenant, id, body)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, out)
}

func (h *Handler) LatestAnalysis(c *gin.Context) {
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	out, err := h.Svc.LatestAnalysis(c.Request.Context(), tenant, id)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, out)
}
func (h *Handler) ListAnalyses(c *gin.Context) {
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	out, err := h.Svc.ListAnalyses(c.Request.Context(), tenant, id)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, gin.H{"list": out})
}
func (h *Handler) CostCenter(c *gin.Context) {
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	out, err := h.Svc.CostCenter(c.Request.Context(), tenant)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, out)
}
func pageData[T any](v PageResult[T]) gin.H {
	return gin.H{"list": v.List, "pagination": gin.H{"page": v.Page, "pageSize": v.PageSize, "total": v.Total, "totalPages": v.TotalPages}}
}
func fail(c *gin.Context, err error) {
	switch {
	case errors.Is(err, ErrNotFound):
		response.Fail(c, 404, response.CodeNotFound, "not found")
	case errors.Is(err, ErrConflict):
		response.Fail(c, 409, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrInvalidTransition):
		response.Fail(c, 409, response.CodeBadRequest, err.Error())
	case errors.Is(err, ErrValidation):
		response.Fail(c, 400, response.CodeBadRequest, err.Error())
	default:
		response.HandleError(c, err)
	}
}

func (h *Handler) ListSources(c *gin.Context) {
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	out, err := h.Svc.ListSources(c.Request.Context(), tenant, pageQuery(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, pageData(out))
}
func (h *Handler) GetSource(c *gin.Context) {
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	out, err := h.Svc.GetSource(c.Request.Context(), tenant, id)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, out)
}
func (h *Handler) CreateSource(c *gin.Context) {
	if !h.write(c) {
		return
	}
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	var body CreateSourceProductBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.CreateSource(c.Request.Context(), tenant, body)
	if err != nil {
		fail(c, err)
		return
	}
	response.JSON(c, 201, response.CodeOK, "ok", out)
}
func (h *Handler) CreateCandidate(c *gin.Context) {
	if !h.write(c) {
		return
	}
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	out, created, err := h.Svc.CreateCandidate(c.Request.Context(), tenant, id)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, gin.H{"candidate": out, "created": created})
}
func (h *Handler) ListCandidates(c *gin.Context) {
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	out, err := h.Svc.ListCandidates(c.Request.Context(), tenant, pageQuery(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, pageData(out))
}
func (h *Handler) GetCandidate(c *gin.Context) {
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	out, err := h.Svc.GetCandidate(c.Request.Context(), tenant, id)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, out)
}
func (h *Handler) ApproveCandidate(c *gin.Context) {
	if !h.write(c) {
		return
	}
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	out, err := h.Svc.ApproveCandidate(c.Request.Context(), tenant, id)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, out)
}
func (h *Handler) RejectCandidate(c *gin.Context) {
	if !h.write(c) {
		return
	}
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body TransitionCandidateBody
	if c.Request.ContentLength > 0 {
		if err := c.ShouldBindJSON(&body); err != nil {
			response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
			return
		}
	}
	out, err := h.Svc.RejectCandidate(c.Request.Context(), tenant, id, body.RejectionReason)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, out)
}
func (h *Handler) WatchCandidate(c *gin.Context) {
	if !h.write(c) {
		return
	}
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	out, err := h.Svc.WatchCandidate(c.Request.Context(), tenant, id)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, out)
}
func (h *Handler) ListCatalog(c *gin.Context) {
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	out, err := h.Svc.ListCatalog(c.Request.Context(), tenant, pageQuery(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, pageData(out))
}
func (h *Handler) GetCatalog(c *gin.Context) {
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	out, err := h.Svc.GetCatalog(c.Request.Context(), tenant, id)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, out)
}
func (h *Handler) CreateListingDraft(c *gin.Context) {
	if !h.write(c) {
		return
	}
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body CreateListingDraftBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, created, err := h.Svc.CreateListingDraft(c.Request.Context(), tenant, id, body)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, gin.H{"listingDraft": out, "created": created})
}
func (h *Handler) ListListingDrafts(c *gin.Context) {
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	out, err := h.Svc.ListListingDrafts(c.Request.Context(), tenant, pageQuery(c))
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, pageData(out))
}
func (h *Handler) GetListingDraft(c *gin.Context) {
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	out, err := h.Svc.GetListingDraft(c.Request.Context(), tenant, id)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, out)
}
func (h *Handler) UpdateListingDraft(c *gin.Context) {
	if !h.write(c) {
		return
	}
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	var body UpdateListingDraftBody
	if err := c.ShouldBindJSON(&body); err != nil {
		response.Fail(c, 400, response.CodeBadRequest, "invalid json body")
		return
	}
	out, err := h.Svc.UpdateListingDraft(c.Request.Context(), tenant, id, body)
	if err != nil {
		fail(c, err)
		return
	}
	response.OK(c, out)
}
func (h *Handler) DeleteListingDraft(c *gin.Context) {
	if !h.write(c) {
		return
	}
	tenant, ok := h.tenant(c)
	if !ok {
		return
	}
	id, ok := parseID(c)
	if !ok {
		return
	}
	if err := h.Svc.DeleteListingDraft(c.Request.Context(), tenant, id); err != nil {
		fail(c, err)
		return
	}
	response.OK(c, gin.H{"deleted": true})
}
