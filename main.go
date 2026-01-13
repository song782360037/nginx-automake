package main

import (
	"embed"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"nginx-automake/internal/job"
	"nginx-automake/internal/modules"
	"nginx-automake/internal/parser"
)

//go:embed web/* config/modules.json
var assets embed.FS

func main() {
	gin.SetMode(gin.ReleaseMode)
	if mode := os.Getenv("GIN_MODE"); mode != "" {
		gin.SetMode(mode)
	}

	modulesData, err := assets.ReadFile("config/modules.json")
	if err != nil {
		panic(err)
	}
	registry, err := modules.LoadRegistry(modulesData)
	if err != nil {
		panic(err)
	}

	workers := getEnvInt("MAX_WORKERS", 2)
	modulesDir := getEnv("MODULES_DIR", "./modules")
	workRoot := getEnv("WORKDIR", "/tmp/nginx-build")
	timeout := getEnvDuration("BUILD_TIMEOUT", 90*time.Minute)
	historyPath := getEnv("HISTORY_FILE", "./data/history.json")

	historyStore, err := job.NewHistoryStore(historyPath)
	if err != nil {
		panic(err)
	}

	queue := job.NewQueue(workers, modulesDir, workRoot, registry, timeout, historyStore)
	queue.Start()

	r := gin.Default()
	indexData, err := assets.ReadFile("web/index.html")
	if err != nil {
		panic(err)
	}

	r.GET("/", func(c *gin.Context) {
		c.Data(http.StatusOK, "text/html; charset=utf-8", indexData)
	})

	r.GET("/api/modules", func(c *gin.Context) {
		c.JSON(http.StatusOK, registry.List())
	})

	r.POST("/api/parse", func(c *gin.Context) {
		var payload struct {
			Output string `json:"output"`
		}
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误"})
			return
		}
		result, err := parser.ParseNginxV(payload.Output)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, result)
	})

	r.POST("/api/build", func(c *gin.Context) {
		var payload job.BuildRequest
		if err := c.ShouldBindJSON(&payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": "请求数据格式错误"})
			return
		}
		if err := queue.ValidateRequest(payload); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		jobItem, err := queue.Enqueue(payload)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
			return
		}
		c.JSON(http.StatusOK, gin.H{"id": jobItem.ID})
	})

	r.GET("/api/jobs/:id", func(c *gin.Context) {
		jobID := c.Param("id")
		jobItem, ok := queue.Get(jobID)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
			return
		}
		c.JSON(http.StatusOK, jobItem)
	})

	r.GET("/api/jobs/:id/download", func(c *gin.Context) {
		jobID := c.Param("id")
		jobItem, ok := queue.Get(jobID)
		if !ok {
			c.JSON(http.StatusNotFound, gin.H{"error": "任务不存在"})
			return
		}
		if jobItem.Status != job.StatusSuccess || jobItem.ArtifactPath == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "产物尚未准备好"})
			return
		}
		filename := "nginx"
		if jobItem.Result != nil && jobItem.Result.Version != "" {
			filename = "nginx-" + jobItem.Result.Version
		}
		c.FileAttachment(jobItem.ArtifactPath, filename)
	})

	r.GET("/api/history", func(c *gin.Context) {
		c.JSON(http.StatusOK, historyStore.List())
	})

	r.GET("/api/history/:id/download", func(c *gin.Context) {
		historyID := c.Param("id")
		entries := historyStore.List()
		for _, entry := range entries {
			if entry.ID == historyID {
				if entry.Artifact == "" {
					c.JSON(http.StatusBadRequest, gin.H{"error": "产物不存在"})
					return
				}
				filename := "nginx"
				if entry.Version != "" {
					filename = "nginx-" + entry.Version
				}
				c.FileAttachment(entry.Artifact, filename)
				return
			}
		}
		c.JSON(http.StatusNotFound, gin.H{"error": "记录不存在"})
	})

	r.GET("/api/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	port := getEnv("PORT", "8080")
	if strings.HasPrefix(port, ":") {
		r.Run(port)
		return
	}
	if err := r.Run(":" + port); err != nil {
		panic(err)
	}
}

func getEnv(key, def string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return def
	}
	return value
}

func getEnvInt(key string, def int) int {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return def
	}
	if parsed, err := strconv.Atoi(value); err == nil {
		return parsed
	}
	return def
}

func getEnvDuration(key string, def time.Duration) time.Duration {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return def
	}
	parsed, err := time.ParseDuration(value)
	if err != nil {
		return def
	}
	return parsed
}
