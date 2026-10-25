package middlewares

import (
	"log/slog"
	"net/http"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

func RequestLogger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		slog.Info("http request", "method", c.Request.Method, "path", c.Request.URL.Path, "status", c.Writer.Status(), "duration", time.Since(start).String(), "client_ip", c.ClientIP())
	}
}

func RateLimit(max int) gin.HandlerFunc {
	type bucket struct {
		count int
		reset time.Time
	}
	var mu sync.Mutex
	clients := map[string]bucket{}
	return func(c *gin.Context) {
		mu.Lock()
		key, now := c.ClientIP(), time.Now()
		entry := clients[key]
		if now.After(entry.reset) {
			entry = bucket{reset: now.Add(time.Minute)}
		}
		entry.count++
		clients[key] = entry
		allowed := entry.count <= max
		if len(clients) > 10000 {
			for client, b := range clients {
				if now.After(b.reset) {
					delete(clients, client)
				}
			}
		}
		mu.Unlock()
		if !allowed {
			c.AbortWithStatusJSON(http.StatusTooManyRequests, gin.H{"error": gin.H{"code": "rate_limit_exceeded", "message": "too many requests"}})
			return
		}
		c.Next()
	}
}
