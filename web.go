package main

import (
	"bytes"
	"embed"
	"fmt"
	"github.com/cfwidget/cfwidget/env"
	"github.com/cfwidget/cfwidget/widget"
	"github.com/gin-gonic/gin"
	"github.com/spf13/cast"
	"go.elastic.co/apm/v2"
	"golang.org/x/text/language"
	"golang.org/x/text/message"
	"gorm.io/gorm"
	"html/template"
	"log"
	"net/http"
	"strings"
	"time"
)

type ApiWebResponse struct {
	Error    string `json:"error,omitempty"`
	Accepted bool   `json:"accepted,omitempty"`
}

const AuthorPath = "author/"

var templateEngine *template.Template
var messagePrinter = message.NewPrinter(language.English)

//go:embed favicon.ico
var faviconFile []byte

//go:embed css/app.css
var cssFile []byte

//go:embed templates/*
var templates embed.FS

func RegisterApiRoutes(e *gin.Engine) {
	var err error
	templateEngine, err = template.New("").ParseFS(templates, "templates/*.tmpl")
	if err != nil {
		panic(err)
	}

	e.SetHTMLTemplate(templateEngine)

	e.GET("/*projectPath", setTransaction, readFromCache, Resolve, GetAuthor, GetProject)
	e.DELETE("/*projectPath", setTransaction, deleteFromCache)
	e.POST("/:id", SyncCall)
}

func Resolve(c *gin.Context) {
	path := strings.TrimSuffix(strings.TrimPrefix(c.Param("projectPath"), "/"), ".json")
	path = strings.TrimSuffix(path, ".png")

	if path == "" {
		//if this is not the web side of the fence, redirect to the web side of the fence
		if c.Request.Host != env.Get("WEB_HOSTNAME") {
			c.Redirect(http.StatusTemporaryRedirect, "https://"+env.Get("WEB_HOSTNAME"))
			return
		} else {
			buf := &bytes.Buffer{}
			_ = templateEngine.ExecuteTemplate(buf, "documentation.tmpl", gin.H{
				"WEB_HOSTNAME": env.Get("WEB_HOSTNAME"),
				"API_HOSTNAME": env.Get("API_HOSTNAME"),
			})
			data := buf.Bytes()

			SetInCache(c.Request.URL.Host, c.Request.URL.RequestURI(), http.StatusOK, "text/html", data)
			c.Data(http.StatusOK, "text/html", data)

			c.Abort()
			return
		}
	}

	if path == "favicon.ico" {
		c.Data(http.StatusOK, "image/x-icon", faviconFile)
		c.Abort()
		return
	} else if path == "css/app.css" {
		c.Data(http.StatusOK, "text/css", cssFile)
		c.Abort()
		return
	} else if path == "service-worker.js" || path == "service-worker-dev.js" || path == "robots.txt" {
		SetInCache(c.Request.URL.Host, c.Request.URL.RequestURI(), http.StatusNotFound, "", nil)
		c.AbortWithStatus(http.StatusNotFound)
		return
	}

	if strings.HasPrefix(path, AuthorPath) {
		handleResolveAuthor(c, strings.TrimPrefix(path, AuthorPath))
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
	loader := c.Query("loader")

	var latest widget.ProjectFile
	for _, v := range properties.Files {
		if v.UploadedAt.After(latest.UploadedAt) {
			if !loaderMatches(loader, v.Versions) {
				continue
			}
			if versionRequest == "" {
				latest = v
			} else if versionRequest == cast.ToString(v.Id) {
				latest = v
			} else if versionRequest == v.Type {
				latest = v
			} else {
				if contains(versionRequest, v.Versions) {
					latest = v
				}
				for _, y := range v.Versions {
					if versionRequest == fmt.Sprintf("%s/%s", y, v.Type) {
						latest = v
						break
					}
				}
			}
		}
	}

	if latest.Id != 0 {
		properties.Download = &latest
	}

	if c.Request.Host == env.Get("API_HOSTNAME") {
		status := project.Status

		//if our status is over 400, check if we have data. If we do, we can use that instead
		if status > 400 {
			if properties != nil {
				status = http.StatusOK
			} else {
				status = http.StatusNotFound
			}
		}

		cacheExpireTime := SetInCache(c.Request.Host, c.Request.URL.RequestURI(), status, "application/json", properties)
		cacheHeaders(c, cacheExpireTime)
		c.JSON(status, properties)
	} else {
		path := strings.TrimSuffix(strings.TrimPrefix(c.Param("projectPath"), "/"), ".json")
		if strings.HasSuffix(path, ".png") {
			_, dark := c.GetQuery("dark")
			_, transparent := c.GetQuery("transparent")
			_, nuThumb := c.GetQuery("noThumbnail")

			imageRequest := ImageRequest{
				DarkMode:    dark,
				Transparent: transparent,
				NoThumbnail: nuThumb,
			}

			data, err := generateImage(properties, imageRequest, c.Request.Context())
			if err != nil {
				log.Print(err)
				c.AbortWithStatus(http.StatusInternalServerError)
				return
			}

			cacheExpireTime := SetInCache(c.Request.Host, c.Request.URL.RequestURI(), http.StatusOK, "image/png", data)
			cacheHeaders(c, cacheExpireTime)

			c.Data(http.StatusOK, "image/png", data)
		} else {
			downloads := messagePrinter.Sprintf("%d\n", properties.Downloads["total"])

			buf := &bytes.Buffer{}
			_ = templateEngine.ExecuteTemplate(buf, "widget.tmpl", gin.H{
				"project":       properties,
				"downloadCount": downloads,
			})
			data := buf.Bytes()

			cacheExpireTime := SetInCache(c.Request.Host, c.Request.URL.RequestURI(), http.StatusOK, "text/html", data)
			cacheHeaders(c, cacheExpireTime)
			c.Data(http.StatusOK, "text/html", data)
		}
	}
	c.Abort()
}

func GetAuthor(c *gin.Context) {
	obj, exists := c.Get("author")
	if !exists {
		return
	}

	author := obj.(*widget.Author)
	response := widget.AuthorResponse{
		Id:       author.MemberId,
		Username: author.Username,
		Projects: author.ParsedProjects.Projects,
	}

	cacheExpireTime := SetInCache(c.Request.Host, c.Request.URL.RequestURI(), 200, "application/json", response)
	cacheHeaders(c, cacheExpireTime)

	c.JSON(http.StatusOK, response)
}

func SyncCall(c *gin.Context) {
	id := strings.TrimPrefix(c.Param("id"), "/")
	SyncProject(cast.ToUint(id), c.Request.Context())
	c.Status(http.StatusNoContent)
}

func handleResolveProject(c *gin.Context, path string) {
	db, err := GetDatabase()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, ApiWebResponse{Error: err.Error()})
		return
	}

	ctx := c.Request.Context()

	db = db.WithContext(ctx)

	if strings.HasPrefix(path, "mc-mods/minecraft/") {
		path = "minecraft/mc-mods/" + strings.TrimPrefix(path, "mc-mods/minecraft/")
	}

	lookup := &widget.ProjectLookup{Path: path}

	if id, err := cast.ToUintE(path); err == nil {
		//the url is actually the id, so can provide the JSON directly
		//this also fixes the author endpoint when you query with that ID
		lookup.CurseId = &id
	} else {
		//the path given is just a path, we need to resolve it to a project
		err = db.Where(lookup).First(&lookup).Error

		if err == gorm.ErrRecordNotFound {
			lookup.CurseId = addProjectConsumer.Consume(path, ctx)
			err = db.Save(&lookup).Error
			if err != nil {
				c.AbortWithStatusJSON(http.StatusInternalServerError, ApiWebResponse{Error: err.Error()})
			}
		} else if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, ApiWebResponse{Error: err.Error()})
			return
		}

		if lookup.CurseId == nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	}

	project := &widget.Project{
		CurseId: *lookup.CurseId,
	}
	err = db.First(&project).Error

	if err == gorm.ErrRecordNotFound || project.ParsedProjects == nil || project.UpdatedAt.Before(time.Now().Add(-1*time.Hour)) {
		project = SyncProject(project.CurseId, ctx)
	} else if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, ApiWebResponse{Error: err.Error()})
		return
	}

	if project == nil || project.CurseId == 0 {
		c.AbortWithStatus(http.StatusNotFound)
		return
  } 
  
	switch project.Status {
	case 404:
		{
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
	case 403:
		fallthrough
	case 200:
		c.Set("project", project)
	default:
		c.AbortWithStatusJSON(http.StatusInternalServerError, ApiWebResponse{Error: fmt.Sprintf("project status is unknown (%d)", project.Status)})
	}
}

func handleResolveAuthor(c *gin.Context, path string) {
	ctx := c.Request.Context()

	db, err := GetDatabase()
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, ApiWebResponse{Error: err.Error()})
		return
	}

	db = db.WithContext(ctx)

	author := &widget.Author{}

	if strings.HasPrefix(path, "search/") {
		username := strings.TrimPrefix(path, "search/")
		err = db.Where("username = ?", username).First(&author).Error
	} else {
		var id uint
		id, err = cast.ToUintE(path)
		if err != nil {
			c.AbortWithStatus(http.StatusNotFound)
			return
		}
		err = db.Where("member_id = ?", id).First(&author).Error
	}

	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, ApiWebResponse{Error: err.Error()})
		return
	}

	if author.UpdatedAt.Before(time.Now().Add(-1 * time.Hour)) {
		temp := syncAuthorConsumer.Consume(author.MemberId, ctx)
		if temp != nil {
			author = temp
		}
	}

	c.Set("author", author)
}

func loaderMatches(loader string, versions []string) bool {
	if loader == "" {
		return true
	}
	return contains(loader, versions)
}

func cacheHeaders(c *gin.Context, cacheExpireTime time.Time) {
	maxAge := cacheTtl.Seconds()
	age := cacheTtl.Seconds() - cacheExpireTime.Sub(time.Now()).Seconds()

	c.Header("Cache-Control", fmt.Sprintf("max-age=%.0f, public", maxAge))
	c.Header("Age", fmt.Sprintf("%.0f", age))
	c.Header("MemCache-Expires-At", cacheExpireTime.UTC().Format(time.RFC3339))
}

func readFromCache(c *gin.Context) {
	trans := apm.TransactionFromContext(c.Request.Context())

	cacheData, exists := GetFromCache(c.Request.Host, c.Request.URL.RequestURI())
	if exists {
		cacheHeaders(c, cacheData.ExpireAt)

		if trans != nil {
			trans.TransactionData.Context.SetLabel("cached", true)
		}

		if cacheData.ContentType == "application/json" {
			c.JSON(cacheData.Status, cacheData.Data)
		} else {
			data, ok := cacheData.Data.([]byte)
			if ok {
				c.Data(cacheData.Status, cacheData.ContentType, data)
			}
		}

		c.Abort()
	} else {
		if trans != nil {
			trans.TransactionData.Context.SetLabel("cached", false)
		}
	}
}

func setTransaction(c *gin.Context) {
	trans := apm.TransactionFromContext(c.Request.Context())
	if trans != nil {
		for k, v := range c.Request.URL.Query() {
			trans.TransactionData.Context.SetLabel(k, strings.ToLower(strings.Join(v, ",")))
		}

		for _, v := range c.Params {
			trans.TransactionData.Context.SetLabel(v.Key, v.Value)
		}
	}
}

func deleteFromCache(c *gin.Context) {
	RemoveFromCache(c.Request.Host, c.Request.URL.RequestURI())
	c.Status(http.StatusAccepted)
}
