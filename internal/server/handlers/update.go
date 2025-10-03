package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/stepanov-ds/GophKeeper/internal/server/database"
	"github.com/stepanov-ds/GophKeeper/internal/utils/structs"
)

func Update(c *gin.Context) {
	var bodyJSON struct {
		ID       int64           `json:"ID,omitempty"`
		Type     string          `json:"type"`
		Data     string          `json:"data,omitempty"`
		Metadata json.RawMessage `json:"metadata,omitempty"`
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

	var err error
	var secureDataID int64
	var historyID int64

	ctx := c.Request.Context()

	switch bodyJSON.Type {
	case "ADD":
		secureDataID, historyID, err = database.AddSecureData(ctx, login, bodyJSON.Data, string(bodyJSON.Metadata))
		if err != nil {
			err = fmt.Errorf("error while add secure data in db: %w", err)
		} else {
			c.JSON(http.StatusOK, structs.Response{
				Message: "ADD success",
				SecureDataID: secureDataID,
				HistoryID: historyID,
			})
		}
		
	case "DELETE":
		historyID, err = database.DeleteSecureData(ctx, bodyJSON.ID, login)
		if err != nil {
			err = fmt.Errorf("error while delete secure data from db: %w", err)
		} else {
			c.JSON(http.StatusOK, structs.Response{
				Message: "DELETE success",
				HistoryID: historyID,
			})
		}
	case "UPDATE":
		historyID, err =  database.UpdateSecureData(ctx, bodyJSON.ID, login, bodyJSON.Data, string(bodyJSON.Metadata))
		if err != nil {
			err = fmt.Errorf("error while update secure data from db: %w", err)
		} else {
			c.JSON(http.StatusOK, structs.Response{
				Message: "UPDATE success",
				HistoryID: historyID,
			})
		}
	default:
		err = fmt.Errorf("type variable must be ADD, UPDATE or DELETE")
	}
	if err != nil {
		c.Error(err)
		c.JSON(http.StatusBadRequest, structs.Response{
			Error: err.Error(),
		})
		return
	}

	

}
