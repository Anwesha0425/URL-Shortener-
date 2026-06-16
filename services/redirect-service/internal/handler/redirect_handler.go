package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
	"go.uber.org/zap"
	"github.com/Anwesha0425/url-shortener-microservice/redirect-service/internal/service"
)

type RedirectHandler struct {
	svc    *service.RedirectService
	logger *zap.Logger
}

func NewRedirectHandler(svc *service.RedirectService, logger *zap.Logger) *RedirectHandler {
	return &RedirectHandler{svc: svc, logger: logger}
}

// Redirect handles GET /:short_code — the hot path.
// Target latency: p99 < 50ms.
func (h *RedirectHandler) Redirect(c *gin.Context) {
	shortCode := c.Param("short_code")
	if shortCode == "" {
		c.Status(http.StatusNotFound)
		return
	}

	originalURL, err := h.svc.Resolve(c.Request.Context(), shortCode)
	if err != nil {
		h.logger.Debug("short code not found", zap.String("code", shortCode))
		c.JSON(http.StatusNotFound, gin.H{"error": "URL not found"})
		return
	}

	c.Redirect(http.StatusFound, originalURL)
}
