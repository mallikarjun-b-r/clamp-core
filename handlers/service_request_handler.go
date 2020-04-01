package handlers

import (
	. "clamp-core/models"
	"clamp-core/services"
	"github.com/gin-gonic/gin"
	"log"
	"net/http"
)

func createServiceRequestHandler() gin.HandlerFunc {
	return func(c *gin.Context) {
		workflowName := c.Param("workflow")
		workflow, err := services.FindWorkflowByName(workflowName)
		if err != nil {
			c.JSON(http.StatusBadRequest, "No record found with given workflow name : " + workflowName)
			return
		}
		log.Println("Loaded workflow -", workflow)
		// Create new service request
		serviceReq := NewServiceRequest(workflowName)
		serviceReq, _ = services.SaveServiceRequest(serviceReq)
		services.AddServiceRequestToChannel(serviceReq)
		//TODO - handle error scenario. Currently it is always 200 ok
		c.JSON(http.StatusOK, serviceReq)
	}
}
