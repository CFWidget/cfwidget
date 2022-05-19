package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"github.com/lordralex/cfwidget/curseforge"
	"github.com/lordralex/cfwidget/env"
	"go.elastic.co/apm/module/apmgin/v2"
	"go.elastic.co/apm/v2"
	"golang.org/x/sync/errgroup"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var g errgroup.Group

func init() {
	if env.Get("CORE_KEY") == "" {
		panic(errors.New("CORE_KEY OR CORE_KEY_FILE MUST BE DEFINED"))
	}
}

func main() {
	//run actual website
	webServer := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	//there is a race condition where APM doesn't handle creating "default" right twice
	_ = apm.DefaultTracer()

	g.Go(func() error {
		web := gin.New()
		web.Use(apmgin.Middleware(web))

		web.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
			if param.Latency > time.Minute {
				param.Latency = param.Latency - param.Latency%time.Second
			}

			return fmt.Sprintf("[GIN] %v | %3d | %13v | %15s | %s | %s | %#v \n%s",
				param.TimeStamp.Format("2006/01/02 - 15:04:05"),
				param.StatusCode, param.Latency,
				param.ClientIP,
				param.Method,
				param.Request.Host,
				param.Path,
				param.ErrorMessage,
			)
		}))

		web.Use(cors.New(cors.Config{
			AllowAllOrigins:  true,
			AllowMethods:     []string{"GET"},
			AllowHeaders:     []string{"Origin", "Content-Length", "Content-Type"},
			AllowCredentials: false,
			MaxAge:           12 * time.Hour,
		}))

		RegisterApiRoutes(web)
		webServer.Handler = web

		log.Printf("Starting web services\n")
		err := webServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
		return nil
	})

	curseforge.StartGameCacheSyncer()

	go func() {
		ticker := time.NewTicker(time.Minute)

		ScheduleAuthors()
		for {
			select {
			case <-ticker.C:
				ScheduleAuthors()
			}
		}
	}()

	quit := make(chan os.Signal)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	log.Println("Shutting down server...")

	shutdownServer(webServer)

	if err := g.Wait(); err != nil {
		log.Fatal(err)
	}
}

func shutdownServer(httpServer *http.Server) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(ctx); err != nil {
		log.Printf("Server forced to shutdown: %s\n", err)
	}
}
