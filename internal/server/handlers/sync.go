package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stepanov-ds/GophKeeper/internal/server/database"
	"github.com/stepanov-ds/GophKeeper/internal/utils/structs"
)

func Sync(c *gin.Context) {
	var bodyJSON struct {
		Last     int64           `json:"lastHistoryID"`
		Limit 	int `json:"limit"`
	}
	if err := c.ShouldBindBodyWithJSON(&bodyJSON); err != nil {
		err = fmt.Errorf("error while parsing JSON: %w", err)
		c.Error(err)
		c.JSON(http.StatusBadRequest, structs.Response{
			Error: err.Error(),
		})
		return
	}

	l, exist := c.Get("login")
	if !exist {
		err := fmt.Errorf("authorization token error")
		c.Error(err)
		c.JSON(http.StatusBadRequest, structs.Response{
			Error: err.Error(),
		})
		return
	}

	login, ok := l.(string)
	if !ok {
		err := fmt.Errorf("type assertion error (login)")
		c.Error(err)
		c.JSON(http.StatusBadRequest, structs.Response{
			Error: err.Error(),
		})
		return
	}

	data, err := database.SelectUpdatedSecureData(bodyJSON.Last, login, bodyJSON.Limit) 
	if err != nil {
		err = fmt.Errorf("error while selecting data from db: %w", err)
		c.Error(err)
		c.JSON(http.StatusInternalServerError, structs.Response{
			Error: err.Error(),
		})
		return
	}

	if len(data) != 0 {
		fullySynced := false
		if len(data) < int(bodyJSON.Limit) {
			fullySynced = true
		}
		c.JSON(http.StatusOK, structs.Response{
			FullySynced: fullySynced,
			SecureData: data,
		})
	}
}
