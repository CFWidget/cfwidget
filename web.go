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
	var err error

	cacheLen := os.Getenv("CACHE_TTL")

	cacheTtl := persistence.DEFAULT
	if cacheLen != "" {
		cacheTtl, err = time.ParseDuration(cacheLen)
		if err != nil {
			panic(err)
		}
	}

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
	c.Header("Cache-Control", "max-age=60")
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

	project := &widget.Project{}
	var err error
	var id uint

	if id, err = cast.ToUintE(path); err == nil {
		//the url is actually the id, so i can provide the JSON directly
		//this also fixes the author endpoint when you query with that ID
		err = db.Where("curse_id = ?", id).Find(&project).Error
	} else {
		//the path given is just a path, we need to resolve it to a project
		err = db.Where("path = ?", path).Find(&project).Error
	}

	//if the record doesn't exist, queue it to be located
	if err == gorm.ErrRecordNotFound {
		//create the record directly, then submit to processor
		project = &widget.Project{
			Path:   path,
			Status: http.StatusOK,
		}

		err = db.Save(&project).Error
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, ApiWebResponse{Error: err.Error()})
			return
		}

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
	case 302:
		{
			//this project is actually pointing elsewhere, we need to find the correct one instead
			err = db.Where("curse_id = ? AND status = 200", project.CurseId).Find(&project).Error
			if err == gorm.ErrRecordNotFound {
				//uh........ how can we have a project redirect but no other project.....
				c.AbortWithStatusJSON(http.StatusInternalServerError, ApiWebResponse{Error: "project indicates redirect but none found"})
				return
			}
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, ApiWebResponse{Error: err.Error()})
				return
			}
		}
	case 200:
		c.Set("project", project)
	case 202:
		c.AbortWithStatusJSON(http.StatusOK, ApiWebResponse{Accepted: true})
	default:
		c.AbortWithStatusJSON(http.StatusInternalServerError, ApiWebResponse{Error: fmt.Sprintf("project status is unknown (%d)", project.Status)})
	}
}

func GetProject(c *gin.Context) {
	obj, _ := c.Get("project")
	project := obj.(*widget.Project)
	properties := project.ParsedProjects

	var latest widget.ProjectFile
	for _, v := range properties.Files {
		if v.UploadedAt.After(latest.UploadedAt) {
			latest = v
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
}
