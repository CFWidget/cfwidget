package web

import (
	"github.com/gin-gonic/gin"
	"github.com/jinzhu/gorm"
	"github.com/lordralex/cfwidget/database"
	"github.com/lordralex/cfwidget/redis"
)

func RegisterApi(e *gin.RouterGroup) {
	e.GET("/*project")
}

func getProject(c *gin.Context) {
	slug := c.Param("project")
	project, err := database.Get(slug)
	if err != nil && !gorm.IsRecordNotFoundError(err) {
		c.Status(500)
		return
	}
	//this means it's a no record, so we have to trigger a lookup
	//we send back the 202 to indicate we'll look at it, then queue it
	if err != nil {
		redis.Submit(slug)
		c.Status(202)
		return
	}

	if project.Id != 0 && project.Status == 200 {
		c.JSON(200, project.Properties)
		return
	} else if project.Id != 0 && project.Status != 200 {
		c.Status(project.Status)
		return
	} else {
		c.Status(404)
		return
	}
}