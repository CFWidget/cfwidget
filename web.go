package main

import (
	"fmt"
	"github.com/gin-contrib/cache"
	"github.com/gin-contrib/cache/persistence"
	"github.com/gin-gonic/gin"
	"github.com/lordralex/cfwidget/widget"
	"github.com/spf13/cast"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gorm.io/gorm"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
)

type ApiWebResponse struct {
	Error    string `json:"error,omitempty"`
	Accepted bool   `json:"accepted,omitempty"`
}

var AllowedFiles = []string{"js/app.js", "favicon.ico", "css/app.css"}

func RegisterApiRoutes(e *gin.Engine) {
	e.LoadHTMLGlob("templates/*.tmpl")

	e.GET("/*projectPath", MemCache(ResolveProject, BrowserCache, GetProject))
}

func MemCache(middleware ...gin.HandlerFunc) gin.HandlerFunc {
	var store persistence.CacheStore
	cacheTtl := getCacheTtl()

	if os.Getenv("MEMCACHE_ENABLE") == "true" {
		store = persistence.NewMemcachedBinaryStore(os.Getenv("MEMCACHE_HOST"), os.Getenv("MEMCACHE_USER"), os.Getenv("MEMCACHE_PASS"), cacheTtl)
	} else if os.Getenv("INMEMCACHE_ENABLE") == "true" {
		store = persistence.NewInMemoryStore(cacheTtl)
	}

	if store != nil {
		return cache.CachePage(store, time.Minute, runHandlers(middleware...))
	}

	return runHandlers(middleware...)
}

func BrowserCache(c *gin.Context) {
	c.Header("Cache-Control", fmt.Sprintf("max-age=%.0f", getCacheTtl().Seconds()))
}

func runHandlers(middleware ...gin.HandlerFunc) gin.HandlerFunc {
	return func(c *gin.Context) {
		for _, v := range middleware {
			v(c)
			if c.IsAborted() {
				break
			}
		}
	}
}

func getCacheTtl() time.Duration {
	var err error
	cacheLen := os.Getenv("CACHE_TTL")

	cacheTtl := persistence.DEFAULT
	if cacheLen != "" {
		cacheTtl, err = time.ParseDuration(cacheLen)
		if err != nil {
			panic(err)
		}
	}

	return cacheTtl
}

func ResolveProject(c *gin.Context) {
	path := strings.TrimPrefix(c.Param("projectPath"), "/")

	if path == "" {
		//if this is not the web side of the fence, redirect to the web side of the fence
		if c.Request.Host != os.Getenv("WEB_HOSTNAME") {
			c.Redirect(http.StatusTemporaryRedirect, "https://"+os.Getenv("WEB_HOSTNAME"))
			c.Abort()
			return
		} else {
			//otherwise, render our documentation
			if pusher := c.Writer.Pusher(); pusher != nil {
				// use pusher.Push() to do server push
				if err := pusher.Push("/css/app.css", nil); err != nil {
					log.Printf("Failed to push: %v", err)
				}
				if err := pusher.Push("/js/app.js", nil); err != nil {
					log.Printf("Failed to push: %v", err)
				}
			}
			c.HTML(http.StatusOK, "documentation.tmpl", gin.H{})
			c.Abort()
			return
		}
	}

	for _, v := range AllowedFiles {
		if v == path {
			if strings.HasSuffix(v, ".js") {
				c.Header("Content-Type", "application/javascript")
			} else if strings.HasSuffix(v, ".css") {
				c.Header("Content-Type", "text/css")
			}
			c.File(v)
			c.Abort()
			return
		}
	}

	if path == "service-worker.js" || path == "service-worker-dev.js" {
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if strings.HasPrefix(path, "authors/") {

	} else {
		handleResolveProject(c, path)
	}
}

func GetProject(c *gin.Context) {
	obj, exists := c.Get("project")
	if !exists {
		return
	}

	project := obj.(*widget.Project)
	properties := project.ParsedProjects

	versionRequest := c.Query("version")

	var latest widget.ProjectFile
	for _, v := range properties.Files {
		if v.UploadedAt.After(latest.UploadedAt) {
			if versionRequest == "" {
				latest = v
			} else if versionRequest == cast.ToString(v.Id) {
				latest = v
			} else if versionRequest == v.Type {
				latest = v
			} else {
				for _, y := range v.Versions {
					if versionRequest == y {
						latest = v
						break
					} else if versionRequest == fmt.Sprintf("%s/%s", y, v.Type) {
						latest = v
					}
				}
			}
		}
	}

	if latest.Id != 0 {
		properties.Download = &latest
	}

	//if this is not the web side of the fence, redirect to the web side of the fence
	if c.Request.Host != os.Getenv("WEB_HOSTNAME") {
		c.JSON(project.Status, properties)
	} else {
		//otherwise, render our documentation
		//this is for HTTP2 support, which can pre-load files for the client
		if pusher := c.Writer.Pusher(); pusher != nil {
			// use pusher.Push() to do server push
			if err := pusher.Push("/css/app.css", nil); err != nil {
				log.Printf("Failed to push: %v", err)
			}
			if err := pusher.Push("/js/app.js", nil); err != nil {
				log.Printf("Failed to push: %v", err)
			}
		}

		p := message.NewPrinter(language.English)
		downloads := p.Sprintf("%d\n", properties.Downloads["total"])

		c.HTML(http.StatusOK, "widget.tmpl", gin.H{
			"project":       properties,
			"downloadCount": downloads,
		})
	}
	c.Abort()
}

func handleResolveProject(c *gin.Context, path string) {
	project := &widget.Project{}
	var err error
	var id uint

	if id, err = cast.ToUintE(path); err == nil {
		//the url is actually the id, so i can provide the JSON directly
		//this also fixes the author endpoint when you query with that ID
		err = db.Where("curse_id = ?", id).Limit(1).Find(&project).Error
	} else {
		//the path given is just a path, we need to resolve it to a project
		err = db.Where("path = ?", path).Find(&project).Error
	}

	//if the record doesn't exist, queue it to be located
	if err == gorm.ErrRecordNotFound || project.ID == 0 {
		//create the record directly, then submit to processor
		project = &widget.Project{
			Path:    path,
			Status:  http.StatusAccepted,
			CurseId: nil,
		}

		err = db.Create(&project).Error
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, ApiWebResponse{Error: err.Error()})
			return
		}

		addChan <- path
		c.AbortWithStatusJSON(http.StatusOK, ApiWebResponse{Accepted: true})
		return
	}

	//if we have a different error, inform caller
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, ApiWebResponse{Error: err.Error()})
		return
	}

	switch project.Status {
	case 404:
		{
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	case 301:
		fallthrough
	case 302:
		{
			//this project is actually pointing elsewhere, we need to find the correct one instead
			redirect := &widget.Project{}
			err = db.Where("curse_id = ? AND status = 200", project.CurseId).First(&redirect).Error
			if err == gorm.ErrRecordNotFound || redirect.ID == 0 {
				//uh........ how can we have a project redirect but no other project.....
				c.AbortWithStatusJSON(http.StatusInternalServerError, ApiWebResponse{Error: "project indicates redirect but none found"})
				return
			}
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, ApiWebResponse{Error: err.Error()})
				return
			}
			c.Set("project", redirect)
		}
	case 200:
		c.Set("project", project)
	case 202:
		c.AbortWithStatusJSON(http.StatusOK, ApiWebResponse{Accepted: true})
	default:
		c.AbortWithStatusJSON(http.StatusInternalServerError, ApiWebResponse{Error: fmt.Sprintf("project status is unknown (%d)", project.Status)})
	}
}

func handleResolveAuthor(c *gin.Context, path string) {

}
