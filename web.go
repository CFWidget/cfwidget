package main

import (
	"bytes"
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

const AuthorPath = "author/"

var templateEngine *template.Template

func RegisterApiRoutes(e *gin.Engine) {
	templates, err := template.New("").ParseGlob("templates/*.tmpl")
	if err != nil {
		panic(err)
	}
	templateEngine = templates
	e.SetHTMLTemplate(templateEngine)

	e.GET("/*projectPath", setTransaction, readFromCache, Resolve, GetAuthor, GetProject)
	e.POST("/:id", SyncCall)
}

func Resolve(c *gin.Context) {
	path := strings.TrimSuffix(strings.TrimPrefix(c.Param("projectPath"), "/"), ".json")

	if path == "" {
		//if this is not the web side of the fence, redirect to the web side of the fence
		if c.Request.Host != env.Get("WEB_HOSTNAME") {
			c.Redirect(http.StatusTemporaryRedirect, "https://"+env.Get("WEB_HOSTNAME"))
			return
		} else {
			buf := &bytes.Buffer{}
			_ = templateEngine.ExecuteTemplate(buf, "documentation.tmpl", gin.H{})
			data := buf.Bytes()

			SetInCache(c.Request.URL.Host, c.Request.URL.RequestURI(), http.StatusOK, "text/html", data)
			c.Data(http.StatusOK, "text/html", data)

			c.Abort()
			return
		}
	}

	for _, v := range AllowedFiles {
		if v == path {
			var contentType string
			if strings.HasSuffix(v, ".js") {
				contentType = "application/javascript"
			} else if strings.HasSuffix(v, ".css") {
				contentType = "text/css"
			} else if strings.HasSuffix(v, ".ico") {
				contentType = "image/x-icon"
			}

			file, _ := os.ReadFile(v)
			SetInCache(c.Request.Host, c.Request.URL.RequestURI(), http.StatusOK, contentType, file)
			c.Data(http.StatusOK, contentType, file)
			c.Abort()
			return
		}
	}

	if path == "service-worker.js" || path == "service-worker-dev.js" || path == "robots.txt" {
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

	//if this is not the web side of the fence, redirect to the web side of the fence
	if c.Request.Host != env.Get("WEB_HOSTNAME") {
		cacheExpireTime := SetInCache(c.Request.Host, c.Request.URL.RequestURI(), project.Status, "application/json", properties)
		cacheHeaders(c, cacheExpireTime)
		c.JSON(project.Status, properties)
	} else {
		p := message.NewPrinter(language.English)
		downloads := p.Sprintf("%d\n", properties.Downloads["total"])

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

	project := &widget.Project{}
	var id uint

	if id, err = cast.ToUintE(path); err == nil {
		//the url is actually the id, so i can provide the JSON directly
		//this also fixes the author endpoint when you query with that ID
		err = db.Where("curse_id = ?", id).First(&project).Error

		if err == gorm.ErrRecordNotFound || project.ID == 0 {
			project.CurseId = &id
		}
	} else {
		//the path given is just a path, we need to resolve it to a project
		err = db.Where("path = ?", path).First(&project).Error
	}

	//if the record doesn't exist, queue it to be located
	if err == gorm.ErrRecordNotFound || project.ID == 0 {
		//create the record directly, then submit to processor
		project.Path = path
		project.Status = http.StatusAccepted

		err = db.Create(&project).Error
		if err != nil {
			c.AbortWithStatusJSON(http.StatusInternalServerError, ApiWebResponse{Error: err.Error()})
			return
		}

		temp := addProjectConsumer.Consume(path, ctx)
		if temp != nil {
			project = temp
		}
	}

	//if we have a different error, inform caller
	if err != nil {
		c.AbortWithStatusJSON(http.StatusInternalServerError, ApiWebResponse{Error: err.Error()})
		return
	}

	//resync project if older than X time
	if project.UpdatedAt.Before(time.Now().Add(-1 * time.Hour)) {
		temp := SyncProject(project.ID, ctx)
		if temp != nil {
			project = temp
		}
	}

	switch project.Status {
	case 403:
		fallthrough
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
		temp := syncAuthorConsumer.Consume(author.Id, ctx)
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
