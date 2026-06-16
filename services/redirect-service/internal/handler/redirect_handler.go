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

	req := service.RedirectRequest{
		IP:        c.ClientIP(),
		UserAgent: c.GetHeader("User-Agent"),
		Referrer:  c.GetHeader("Referer"),
	}

	originalURL, err := h.svc.Resolve(c.Request.Context(), shortCode, req)
	if err != nil {
		h.logger.Debug("short code not found or expired", zap.String("code", shortCode), zap.Error(err))
		c.JSON(http.StatusNotFound, gin.H{"error": "URL not found or expired"})
		return
	}

	c.Redirect(http.StatusFound, originalURL)
}
