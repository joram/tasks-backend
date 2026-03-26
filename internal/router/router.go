package router

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"task-tracker-api/internal/config"
	"task-tracker-api/internal/handlers"
	"task-tracker-api/internal/middleware"
)

func isAllowedOrigin(origin string) bool {
	// Strip scheme
	host := origin
	if after, ok := strings.CutPrefix(host, "https://"); ok {
		host = after
	} else if after, ok := strings.CutPrefix(host, "http://"); ok {
		host = after
	}
	return host == "veilstreamapp.com" || strings.HasSuffix(host, ".veilstreamapp.com") || strings.HasSuffix(host, ".oram.ca")
}

const apkLatestFilename = "task-tracker-latest.apk"
const apkVersionFilename = "version.txt"
const apkContentType = "application/vnd.android.package-archive"

func Setup(db *gorm.DB, cfg *config.Config) *gin.Engine {
	r := gin.Default()

	r.Use(cors.New(cors.Config{
		AllowOriginFunc:  isAllowedOrigin,
		AllowMethods:     []string{"GET", "POST", "PATCH", "DELETE", "OPTIONS"},
		AllowHeaders:     []string{"Authorization", "Content-Type"},
		AllowCredentials: true,
	}))

	r.GET("/health", func(c *gin.Context) {
		c.JSON(200, gin.H{"status": "ok"})
	})

	// List APK versions the server can serve. Web and mobile use this to choose the latest.
	r.GET("/apk/versions", func(c *gin.Context) {
		versionPath := filepath.Join(cfg.APKDir, apkVersionFilename)
		path := filepath.Join(cfg.APKDir, apkLatestFilename)
		versions := []string{}
		if b, err := os.ReadFile(versionPath); err == nil {
			if v := strings.TrimSpace(string(b)); v != "" {
				if _, err := os.Stat(path); err == nil {
					versions = []string{v}
				}
			}
		}
		latest := ""
		if len(versions) > 0 {
			latest = versions[len(versions)-1]
		}
		c.JSON(200, gin.H{"versions": versions, "latest": latest})
	})

	// Serve APK with version in the URL: GET /apk/v/:version (e.g. /apk/v/1.0.2).
	r.GET("/apk/v/:version", func(c *gin.Context) {
		path := filepath.Join(cfg.APKDir, apkLatestFilename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			c.AbortWithStatus(404)
			return
		}
		version := c.Param("version")
		if version == "" {
			c.AbortWithStatus(404)
			return
		}
		downloadName := "task-tracker-v" + version + ".apk"
		c.Header("Content-Type", apkContentType)
		c.FileAttachment(path, downloadName)
	})

	// Redirect /apk/latest to /apk/v/:version so the download URL contains the version.
	r.GET("/apk/latest", func(c *gin.Context) {
		path := filepath.Join(cfg.APKDir, apkLatestFilename)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			c.AbortWithStatus(404)
			return
		}
		version := ""
		if b, err := os.ReadFile(filepath.Join(cfg.APKDir, apkVersionFilename)); err == nil {
			version = strings.TrimSpace(string(b))
		}
		if version == "" {
			version = "latest"
		}
		c.Redirect(302, "/apk/v/"+version)
	})

	auth := handlers.NewAuthHandler(db, cfg)
	r.POST("/auth/login", auth.Login)
	r.POST("/auth/register", auth.Register)

	tasks := handlers.NewTaskHandler(db)
	lists := handlers.NewListsHandler(db)
	labels := handlers.NewLabelsHandler(db)
	protected := r.Group("", middleware.Auth(cfg.JWTSecret, db))
	{
		protected.POST("/auth/change-password", auth.ChangePassword)
		protected.GET("/labels", labels.List)
		protected.POST("/labels", labels.Create)
		protected.PATCH("/labels/:id", labels.Update)
		protected.DELETE("/labels/:id", labels.Delete)
		protected.GET("/lists", lists.List)
		protected.POST("/lists", lists.Create)
		protected.PATCH("/lists/:id", lists.Update)
		protected.DELETE("/lists/:id", lists.Delete)
		protected.GET("/lists/:id/members", lists.ListMembers)
		protected.POST("/lists/:id/members", lists.AddMember)
		protected.DELETE("/lists/:id/members/:user_id", lists.RemoveMember)
		tasksGroup := protected.Group("/tasks")
		{
			tasksGroup.GET("", tasks.List)              // ?list_id=uuid (required) &archived=true | ?label=slug
			tasksGroup.POST("", tasks.Create)
			tasksGroup.PATCH("/reorder", tasks.Reorder)
			tasksGroup.GET("/:id", tasks.Get)
			tasksGroup.PATCH("/:id", tasks.Update)
			tasksGroup.POST("/:id/archive", tasks.Archive)
			tasksGroup.POST("/:id/unarchive", tasks.Unarchive)
			tasksGroup.DELETE("/:id", tasks.Delete)
		}
	}

	return r
}
