package commands

import (
	"bytes"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"

	"github.com/spf13/cobra"
	"github.com/stepanov-ds/GophKeeper/internal/utils/structs"
	"golang.org/x/term"
)

var deleteCmd = &cobra.Command{
	Use:   "delete",
	Short: "delete saved data",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := executeDelete(args); err != nil {
			fmt.Fprintf(os.Stderr, "delete failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("delete success")
	},
}

func init() {
	RootCmd.AddCommand(deleteCmd)
}

func executeDelete(args []string) error {
	fmt.Print("enter your master-password: ")
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return fmt.Errorf("error while reading password: %w", err)
	}
	fmt.Println()
	dataList, err := selectAll(pass)
		if err != nil {
			return err
		}
		for _, data := range dataList {
			argID, err := strconv.ParseInt(args[0], 10, 64)
			if err != nil {
				return fmt.Errorf("error while parsing args: %w", err)
			}
			if data.id == argID {
				mail, err := getCurrentUser()
				if err != nil {
					return fmt.Errorf("error while getting current user, try to use init command: %w", err)
				}
				dbPath, err := getDBPath(mail)
				if err != nil {
					return fmt.Errorf("error while getDBPath: %w", err)
				}
				db, err := sql.Open("sqlite3", dbPath )
				if err != nil {
					return fmt.Errorf("error while sql.Open: %w", err)
				}
				defer db.Close()

				var bodyJSON struct {
					ID       int64           `json:"ID"`
					Type     string          `json:"type"`
				}
				bodyJSON.Type = "DELETE"
				bodyJSON.ID = data.id
			
				filePath, err := getTokenPath(mail)
				if err != nil {
					return err
				}
			
				fileData, err := os.ReadFile(filePath)
				nonce := fileData[0:12]
				encryptedData := fileData[12:]
			
				key, err := deriveKey(pass)
				if err != nil {
					return err
				}
				token, err := decryptData(key, encryptedData, nonce)
				if err != nil {
					return err
				}

				jsonData, err := json.Marshal(bodyJSON)
				if err != nil {
					return err
				}
			
				req, err := http.NewRequest("POST", serverURL+"/update", bytes.NewBuffer(jsonData))
				if err != nil {
					return fmt.Errorf("request error: %w", err)
				}
				req.Header.Set("Content-Type", "application/json")
				req.AddCookie(&http.Cookie{Name: "Authorization", Value: string(token)})
				client := &http.Client{}
				resp, err := client.Do(req)
				if err != nil {
					return fmt.Errorf("request error: %w", err)
				}
				defer resp.Body.Close()
			
				if resp.StatusCode != http.StatusOK {
					body, err := io.ReadAll(resp.Body)
					if err != nil {
						return err
					}
					defer resp.Body.Close()
					return fmt.Errorf("bad responce: %d, responce body: %s", resp.StatusCode, string(body))
				}
				body, err := io.ReadAll(resp.Body)
				if err != nil {
					return err
				}
				defer resp.Body.Close()
				var respJSON structs.Response
			
				json.Unmarshal(body, &respJSON)

				addSQL := `
					UPDATE secure_data
					SET is_active = false, history_id = ?
					WHERE id = ?`
			
				if _, err := db.Exec(addSQL, respJSON.HistoryID, data.id); err != nil {
					return err
				}
			}
		}
	return nil
}