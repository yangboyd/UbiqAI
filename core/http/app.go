package http

import (
	"embed"
	"errors"
	"fmt"
	"net/http"

	"github.com/dave-gray101/v2keyauth"
	"github.com/mudler/LocalAI/pkg/utils"

	"github.com/mudler/LocalAI/core/http/endpoints/localai"
	"github.com/mudler/LocalAI/core/http/endpoints/openai"
	"github.com/mudler/LocalAI/core/http/middleware"
	"github.com/mudler/LocalAI/core/http/routes"

	"github.com/mudler/LocalAI/core/application"
	"github.com/mudler/LocalAI/core/schema"
	"github.com/mudler/LocalAI/core/services"

	"github.com/gofiber/contrib/fiberzerolog"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"github.com/gofiber/fiber/v2/middleware/csrf"
	"github.com/gofiber/fiber/v2/middleware/favicon"
	"github.com/gofiber/fiber/v2/middleware/filesystem"
	"github.com/gofiber/fiber/v2/middleware/recover"

	// swagger handler
	"github.com/rs/zerolog/log"
)

// Embed a directory
//
//go:embed static/*
var embedDirStatic embed.FS

// @title LocalAI API
// @version 2.0.0
// @description The LocalAI Rest API.
// @termsOfService
// @contact.name LocalAI
// @contact.url https://localai.io
// @license.name MIT
// @license.url https://raw.githubusercontent.com/mudler/LocalAI/master/LICENSE
// @BasePath /
// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization

func API(application *application.Application) (*fiber.App, error) {

	fiberCfg := fiber.Config{
		Views:     renderEngine(),
		BodyLimit: application.ApplicationConfig().UploadLimitMB * 1024 * 1024, // this is the default limit of 4MB
		// We disable the Fiber startup message as it does not conform to structured logging.
		// We register a startup log line with connection information in the OnListen hook to keep things user friendly though
		DisableStartupMessage: true,
		// Override default error handler
	}

	if !application.ApplicationConfig().OpaqueErrors {
		// Normally, return errors as JSON responses
		fiberCfg.ErrorHandler = func(ctx *fiber.Ctx, err error) error {
			// Status code defaults to 500
			code := fiber.StatusInternalServerError

			// Retrieve the custom status code if it's a *fiber.Error
			var e *fiber.Error
			if errors.As(err, &e) {
				code = e.Code
			}

			// Send custom error page
			return ctx.Status(code).JSON(
				schema.ErrorResponse{
					Error: &schema.APIError{Message: err.Error(), Code: code},
				},
			)
		}
	} else {
		// If OpaqueErrors are required, replace everything with a blank 500.
		fiberCfg.ErrorHandler = func(ctx *fiber.Ctx, _ error) error {
			return ctx.Status(500).SendString("")
		}
	}

	router := fiber.New(fiberCfg)

	router.Hooks().OnListen(func(listenData fiber.ListenData) error {
		scheme := "http"
		if listenData.TLS {
			scheme = "https"
		}
		log.Info().Str("endpoint", scheme+"://"+listenData.Host+":"+listenData.Port).Msg("LocalAI API is listening! Please connect to the endpoint for API documentation.")
		return nil
	})

	// Have Fiber use zerolog like the rest of the application rather than it's built-in logger
	logger := log.Logger
	router.Use(fiberzerolog.New(fiberzerolog.Config{
		Logger: &logger,
	}))

	// Default middleware config

	if !application.ApplicationConfig().Debug {
		router.Use(recover.New())
	}

	if !application.ApplicationConfig().DisableMetrics {
		metricsService, err := services.NewLocalAIMetricsService()
		if err != nil {
			return nil, err
		}

		if metricsService != nil {
			router.Use(localai.LocalAIMetricsAPIMiddleware(metricsService))
			router.Hooks().OnShutdown(func() error {
				return metricsService.Shutdown()
			})
		}

	}
	// Health Checks should always be exempt from auth, so register these first
	routes.HealthRoutes(router)

	kaConfig, err := middleware.GetKeyAuthConfig(application.ApplicationConfig())
	if err != nil || kaConfig == nil {
		return nil, fmt.Errorf("failed to create key auth config: %w", err)
	}

	// Auth is applied to _all_ endpoints. No exceptions. Filtering out endpoints to bypass is the role of the Filter property of the KeyAuth Configuration
	router.Use(v2keyauth.New(*kaConfig))

	if application.ApplicationConfig().CORS {
		var c func(ctx *fiber.Ctx) error
		if application.ApplicationConfig().CORSAllowOrigins == "" {
			c = cors.New()
		} else {
			c = cors.New(cors.Config{AllowOrigins: application.ApplicationConfig().CORSAllowOrigins})
		}

		router.Use(c)
	}

	if application.ApplicationConfig().CSRF {
		log.Debug().Msg("Enabling CSRF middleware. Tokens are now required for state-modifying requests")
		router.Use(csrf.New())
	}

	// Load config jsons
	utils.LoadConfig(application.ApplicationConfig().UploadDir, openai.UploadedFilesFile, &openai.UploadedFiles)
	utils.LoadConfig(application.ApplicationConfig().ConfigsDir, openai.AssistantsConfigFile, &openai.Assistants)
	utils.LoadConfig(application.ApplicationConfig().ConfigsDir, openai.AssistantsFileConfigFile, &openai.AssistantFiles)

	galleryService := services.NewGalleryService(application.ApplicationConfig())
	galleryService.Start(application.ApplicationConfig().Context, application.BackendLoader())

	routes.RegisterElevenLabsRoutes(router, application.BackendLoader(), application.ModelLoader(), application.ApplicationConfig())
	routes.RegisterLocalAIRoutes(router, application.BackendLoader(), application.ModelLoader(), application.ApplicationConfig(), galleryService)
	routes.RegisterOpenAIRoutes(router, application)
	if !application.ApplicationConfig().DisableWebUI {
		routes.RegisterUIRoutes(router, application.BackendLoader(), application.ModelLoader(), application.ApplicationConfig(), galleryService)
	}
	routes.RegisterJINARoutes(router, application.BackendLoader(), application.ModelLoader(), application.ApplicationConfig())

	httpFS := http.FS(embedDirStatic)

	router.Use(favicon.New(favicon.Config{
		URL:        "/favicon.ico",
		FileSystem: httpFS,
		File:       "static/favicon.ico",
	}))

	router.Use("/static", filesystem.New(filesystem.Config{
		Root:       httpFS,
		PathPrefix: "static",
		Browse:     true,
	}))

	// Define a custom 404 handler
	// Note: keep this at the bottom!
	router.Use(notFoundHandler)

	return router, nil
}