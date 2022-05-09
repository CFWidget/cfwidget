package main

import (
	"context"
	"errors"
	"fmt"
	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
	"golang.org/x/sync/errgroup"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"
)

var g errgroup.Group

func main() {
	if (os.Getenv("CORE_KEY") == "" || os.Getenv("CORE_KEY") == "${CORE_KEY}") && os.Getenv("CORE_KEY_FILE") == "" {
		panic(errors.New("CORE_KEY OR CORE_KEY_FILE MUST BE DEFINED"))
	}

	if os.Getenv("CORE_KEY_FILE") != "" {
		key, err := readSecret("CORE_KEY_FILE")
		if err != nil {
			panic(err)
		}

		err = os.Setenv("CORE_KEY", key)
		if err != nil {
			panic(err)
		}
	}

	//run actual website
	webServer := &http.Server{
		Addr:         ":8080",
		ReadTimeout:  5 * time.Second,
		WriteTimeout: 10 * time.Second,
	}

	g.Go(func() error {
		web := gin.New()
		web.Use(gin.Recovery())
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

		cors.Default()

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

	if os.Getenv("SYNC_ENABLED") == "true" {
		go func() {
			ticker := time.NewTicker(time.Minute)

			ScheduleProjects()
			for {
				select {
				case <-ticker.C:
					ScheduleProjects()
				}
			}
		}()

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
	}

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

func readSecret(file string) (string, error) {
	f, err := os.Open(os.Getenv(file))
	if err != nil {
		return "", err
	}
	defer f.Close()

	data, err := io.ReadAll(f)
	if err != nil {
		return "", err
	}

	return strings.TrimSpace(string(data)), nil
}
