package productflow

import "github.com/gin-gonic/gin"

func Register(g *gin.RouterGroup, h *Handler) {
	if g == nil || h == nil {
		return
	}
	g.GET("/source-products", h.ListSources)
	g.POST("/source-products", h.CreateSource)
	g.GET("/source-products/:id", h.GetSource)
	g.POST("/source-products/:id/candidate", h.CreateCandidate)
	g.GET("/candidates", h.ListCandidates)
	g.GET("/candidates/:id", h.GetCandidate)
	g.POST("/candidates/:id/analyze", h.AnalyzeCandidate)
	g.GET("/candidates/:id/analysis", h.LatestAnalysis)
	g.GET("/candidates/:id/analyses", h.ListAnalyses)
	g.POST("/candidates/:id/approve", h.ApproveCandidate)
	g.POST("/candidates/:id/reject", h.RejectCandidate)
	g.POST("/candidates/:id/watch", h.WatchCandidate)
	g.GET("/catalog-products", h.ListCatalog)
	g.GET("/catalog-products/:id", h.GetCatalog)
	g.POST("/catalog-products/:id/listing-drafts", h.CreateListingDraft)
	g.GET("/listing-drafts", h.ListListingDrafts)
	g.GET("/listing-drafts/:id", h.GetListingDraft)
	g.PUT("/listing-drafts/:id", h.UpdateListingDraft)
	g.DELETE("/listing-drafts/:id", h.DeleteListingDraft)
	g.GET("/cost-center", h.CostCenter)
}
