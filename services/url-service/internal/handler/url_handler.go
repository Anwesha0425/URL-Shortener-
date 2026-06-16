package handler

import (
	"errors"
	"net/http"
	"strconv"

	"github.com/Anwesha0425/url-shortener-microservice/url-service/internal/domain"
	"github.com/Anwesha0425/url-shortener-microservice/url-service/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"go.uber.org/zap"
)

// URLHandler handles HTTP requests for URL management
type URLHandler struct {
	svc    *service.URLService
	logger *zap.Logger
}

func NewURLHandler(svc *service.URLService, logger *zap.Logger) *URLHandler {
	return &URLHandler{svc: svc, logger: logger}
}

// MetricsHandler returns the Prometheus metrics handler
func MetricsHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// CreateURL godoc
// @Summary Create a short URL
// @Accept json
// @Produce json
// @Param request body domain.CreateURLRequest true "URL creation request"
// @Success 201 {object} domain.CreateURLResponse
// @Router /api/v1/urls [post]
func (h *URLHandler) CreateURL(c *gin.Context) {
	var req domain.CreateURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid request body", err.Error()))
		return
	}

	// In production, extract userID from JWT token
	// For now, use a demo user ID
	userID := int64(1)

	resp, err := h.svc.CreateURL(c.Request.Context(), &req, &userID)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrInvalidURL):
			c.JSON(http.StatusBadRequest, errorResponse("invalid URL", err.Error()))
		case errors.Is(err, service.ErrAliasConflict):
			c.JSON(http.StatusConflict, errorResponse("alias already taken", err.Error()))
		default:
			h.logger.Error("failed to create url", zap.Error(err))
			c.JSON(http.StatusInternalServerError, errorResponse("internal error", ""))
		}
		return
	}

	c.JSON(http.StatusCreated, resp)
}

// GetURL retrieves a URL by ID
func (h *URLHandler) GetURL(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid id", err.Error()))
		return
	}

	url, err := h.svc.GetURL(c.Request.Context(), id)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrURLNotFound):
			c.JSON(http.StatusNotFound, errorResponse("url not found", ""))
		case errors.Is(err, service.ErrURLExpired):
			c.JSON(http.StatusGone, errorResponse("url has expired", ""))
		default:
			c.JSON(http.StatusInternalServerError, errorResponse("internal error", ""))
		}
		return
	}

	c.JSON(http.StatusOK, url)
}

// UpdateURL modifies an existing URL
func (h *URLHandler) UpdateURL(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid id", err.Error()))
		return
	}

	var req domain.UpdateURLRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid request body", err.Error()))
		return
	}

	url, err := h.svc.UpdateURL(c.Request.Context(), id, &req)
	if err != nil {
		switch {
		case errors.Is(err, service.ErrURLNotFound):
			c.JSON(http.StatusNotFound, errorResponse("url not found", ""))
		case errors.Is(err, service.ErrInvalidURL):
			c.JSON(http.StatusBadRequest, errorResponse("invalid URL", err.Error()))
		default:
			c.JSON(http.StatusInternalServerError, errorResponse("internal error", ""))
		}
		return
	}

	c.JSON(http.StatusOK, url)
}

// DeleteURL soft-deletes a URL
func (h *URLHandler) DeleteURL(c *gin.Context) {
	id, err := parseIDParam(c)
	if err != nil {
		c.JSON(http.StatusBadRequest, errorResponse("invalid id", err.Error()))
		return
	}

	if err := h.svc.DeleteURL(c.Request.Context(), id); err != nil {
		if errors.Is(err, service.ErrURLNotFound) {
			c.JSON(http.StatusNotFound, errorResponse("url not found", ""))
			return
		}
		c.JSON(http.StatusInternalServerError, errorResponse("internal error", ""))
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": "url deleted successfully"})
}

// ListURLs returns paginated URLs for the authenticated user
func (h *URLHandler) ListURLs(c *gin.Context) {
	page, _ := strconv.Atoi(c.DefaultQuery("page", "1"))
	pageSize, _ := strconv.Atoi(c.DefaultQuery("page_size", "20"))

	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 100 {
		pageSize = 20
	}

	// In production, extract from JWT
	userID := int64(1)

	urls, total, err := h.svc.ListURLs(c.Request.Context(), userID, page, pageSize)
	if err != nil {
		c.JSON(http.StatusInternalServerError, errorResponse("internal error", ""))
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"data":      urls,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ── Helpers ────────────────────────────────────────────────────────────────

func parseIDParam(c *gin.Context) (int64, error) {
	return strconv.ParseInt(c.Param("id"), 10, 64)
}

func errorResponse(message, detail string) gin.H {
	resp := gin.H{"error": message}
	if detail != "" {
		resp["detail"] = detail
	}
	return resp
}
