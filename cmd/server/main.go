package main

import (
	"context"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/redgreat/apiwong/internal/config"
	"github.com/redgreat/apiwong/internal/database"
	"github.com/redgreat/apiwong/internal/handlers"
	"github.com/redgreat/apiwong/internal/middleware"
	"github.com/redgreat/apiwong/internal/migrations"
	"github.com/redgreat/apiwong/internal/scheduler"
	"github.com/redgreat/apiwong/internal/services"
	"github.com/redgreat/apiwong/internal/utils"
)

func main() {
	if err := config.LoadConfig("configs/config.yaml"); err != nil {
		log.Fatalf("加载配置失败: %v", err)
	}

	if config.AppConfig.Log.OutputPath != "" {
		logDir := filepath.Dir(config.AppConfig.Log.OutputPath)
		if logDir != "." {
			if err := os.MkdirAll(logDir, 0755); err != nil {
				log.Fatalf("创建日志目录失败: %v", err)
			}
		}
		file, err := os.OpenFile(config.AppConfig.Log.OutputPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			log.Fatalf("打开日志文件失败: %v", err)
		}
		log.SetOutput(io.MultiWriter(os.Stdout, file))
	}

	manager := database.GetManager()
	for name, cfg := range config.AppConfig.Databases {
		if err := manager.AddConnection(name, cfg); err != nil {
			log.Fatalf("添加数据库连接失败: %v", err)
		}
	}

	systemDB, err := manager.GetConnection("system")
	if err != nil {
		log.Fatalf("获取系统数据库连接失败: %v", err)
	}

	migrator := &migrations.Migrator{}
	if err := migrator.Run(systemDB); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	connectionService := services.NewConnectionService()
	if err := connectionService.LoadEnabledConnections(); err != nil {
		log.Printf("加载数据库连接失败: %v", err)
	}

	if err := scheduler.GetScheduler().Start(); err != nil {
		log.Fatalf("启动定时任务失败: %v", err)
	}
	services.GetCDCManager().StartAll()

	if config.AppConfig.Server.Mode != "" {
		gin.SetMode(config.AppConfig.Server.Mode)
	}

	router := gin.New()
	router.Use(middleware.Logger(), middleware.CORS(), gin.Recovery())

	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "ok")
	})

	authHandler := handlers.NewAuthHandler()
	dbHandler := handlers.NewDatabaseHandler()
	syncHandler := handlers.NewSyncHandler()
	connectionHandler := handlers.NewConnectionHandler()
	alertHandler := handlers.NewAlertHandler()

	api := router.Group("/api")
	authGroup := api.Group("/auth")
	authGroup.POST("/login", authHandler.Login)

	api.GET("/profile", middleware.AuthMiddleware(), authHandler.GetProfile)
	api.PUT("/profile", middleware.AuthMiddleware(), authHandler.UpdateProfile)
	api.PUT("/profile/password", middleware.AuthMiddleware(), authHandler.ChangePassword)

	userGroup := api.Group("/users", middleware.AuthMiddleware(), middleware.AdminMiddleware())
	userGroup.GET("", authHandler.ListUsers)
	userGroup.POST("", authHandler.CreateUser)
	userGroup.PUT("/:id", authHandler.UpdateUser)
	userGroup.DELETE("/:id", authHandler.DeleteUser)

	dbGroup := api.Group("/db", middleware.AuthMiddleware())
	dbGroup.GET("/connections", connectionHandler.ListConnections)
	dbGroup.GET("/connections/:id", connectionHandler.GetConnection)
	dbGroup.GET("/:name/tables", dbHandler.ListTables)
	dbGroup.GET("/:name/table/:table/schema", dbHandler.GetTableSchema)
	dbAdmin := dbGroup.Group("", middleware.AdminMiddleware())
	dbAdmin.POST("/connections", connectionHandler.CreateConnection)
	dbAdmin.PUT("/connections/:id", connectionHandler.UpdateConnection)
	dbAdmin.DELETE("/connections/:id", connectionHandler.DeleteConnection)
	dbAdmin.POST("/connections/:id/test", connectionHandler.TestConnection)
	dbAdmin.POST("/:name/query", dbHandler.Query)
	dbAdmin.POST("/:name/exec", dbHandler.Exec)
	dbAdmin.POST("/:name/table/:table/data", dbHandler.InsertData)
	dbAdmin.PUT("/:name/table/:table/data/:id", dbHandler.UpdateData)
	dbAdmin.DELETE("/:name/table/:table/data/:id", dbHandler.DeleteData)

	syncGroup := api.Group("/sync", middleware.AuthMiddleware())
	syncGroup.GET("/tasks", syncHandler.ListTasks)
	syncGroup.GET("/tasks/:id", syncHandler.GetTask)
	syncGroup.GET("/tasks/:id/logs", syncHandler.GetTaskLogs)
	syncGroup.GET("/logs", syncHandler.ListLogs)
	syncAdmin := syncGroup.Group("", middleware.AdminMiddleware())
	syncAdmin.POST("/tasks", syncHandler.CreateTask)
	syncAdmin.PUT("/tasks/:id", syncHandler.UpdateTask)
	syncAdmin.DELETE("/tasks/:id", syncHandler.DeleteTask)
	syncAdmin.POST("/tasks/:id/execute", syncHandler.ExecuteTask)
	syncAdmin.POST("/tasks/:id/precheck", syncHandler.PrecheckTask)
	syncAdmin.POST("/tasks/:id/pause", syncHandler.PauseTask)
	syncAdmin.POST("/tasks/:id/resume", syncHandler.ResumeTask)
	syncAdmin.PUT("/tasks/:id/checkpoint", syncHandler.UpdateCheckpoint)

	alertGroup := api.Group("/alerts", middleware.AuthMiddleware())
	alertGroup.GET("/channels", alertHandler.List)
	alertAdmin := alertGroup.Group("", middleware.AdminMiddleware())
	alertAdmin.POST("/channels", alertHandler.Create)
	alertAdmin.PUT("/channels/:id", alertHandler.Update)
	alertAdmin.DELETE("/channels/:id", alertHandler.Delete)
	alertAdmin.POST("/channels/:id/test", alertHandler.Test)

	staticPath := filepath.Join("web", "dist")
	if _, err := os.Stat(staticPath); err == nil {
		faviconPath := filepath.Join(staticPath, "favicon.png")
		if _, err := os.Stat(faviconPath); err == nil {
			router.StaticFile("/favicon.png", faviconPath)
		}
		assetsPath := filepath.Join(staticPath, "assets")
		if _, err := os.Stat(assetsPath); err == nil {
			router.Static("/assets", assetsPath)
		}
		router.GET("/", func(c *gin.Context) {
			c.File(filepath.Join(staticPath, "index.html"))
		})
		router.NoRoute(func(c *gin.Context) {
			if strings.HasPrefix(c.Request.URL.Path, "/api") {
				utils.Error(c, 404, "接口不存在")
				return
			}
			c.File(filepath.Join(staticPath, "index.html"))
		})
	}

	port := config.AppConfig.Server.Port
	if port == "" {
		port = "8080"
	}

	server := &http.Server{
		Addr:    ":" + port,
		Handler: router,
	}

	go func() {
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("启动服务失败: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGTERM)
	<-quit

	scheduler.GetScheduler().Stop()
	services.GetCDCManager().Close()
	manager.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := server.Shutdown(ctx); err != nil {
		log.Printf("服务关闭失败: %v", err)
	}
}
