package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stepanov-ds/GophKeeper/internal/database"
	"github.com/stepanov-ds/GophKeeper/internal/utils/structs"
)

func Register(c *gin.Context) {
	var bodyJSON struct {
		Mail string `json:"mail"`
	}
	if err := c.ShouldBindBodyWithJSON(&bodyJSON); err != nil {
		err = fmt.Errorf("error while parsing JSON: %w", err)
		c.Error(err)
		c.JSON(http.StatusBadRequest, structs.Response{
			Error: err.Error(),
		})
		return
	}
	if err := database.RegisterUser(bodyJSON.Mail); err != nil {
		err = fmt.Errorf("error while inserting user in DB: %w", err)
		c.Error(err)
		c.JSON(http.StatusBadRequest, structs.Response{
			Error: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, structs.Response{
		Message: "registration success",
	})

}
