package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"
)

var g errgroup.Group

func main() {
	if os.Getenv("CORE_KEY") == "" || os.Getenv("CORE_KEY") == "${CORE_KEY}" {
		panic(errors.New("CORE_KEY MUST BE DEFINED"))
	}

	//run actual website
	webServer := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	updateGameCache()

	g.Go(func() error {
		web := gin.New()
		web.Use(gin.Recovery())
		web.Use(gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {
			if param.Latency > time.Minute {
				// Truncate in a golang < 1.8 safe way
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

		RegisterApiRoutes(web)
		webServer.Handler = web

		log.Printf("Starting web services\n")
		err := webServer.ListenAndServe()
		if err != nil && err != http.ErrServerClosed {
			log.Fatal(err)
		}
		return nil
	})

	go func() {
		ticker := time.NewTicker(time.Minute)
		for {
			select {
			case <-ticker.C:
				ScheduleProjects()
			}
		}
	}()

	go func() {
		ticker := time.NewTicker(time.Hour)
		for {
			select {
			case <-ticker.C:
				updateGameCache()
			}
		}
	}()

	//SyncProject(17618)

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
