package handlers

import (
	"fmt"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stepanov-ds/GophKeeper/internal/server/database"
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
		if pgErr, ok := err.(*pgconn.PgError); ok {
			const uniqueViolationCode = "23505" 
			
			if pgErr.Code == uniqueViolationCode {
				c.JSON(http.StatusConflict, structs.Response{
					Message: "user already registred",
				})
				return 
			}
		}
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
