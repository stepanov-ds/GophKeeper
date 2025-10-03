package commands

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "list saved data",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		if err := executeList(args); err != nil {
			fmt.Fprintf(os.Stderr, "list failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("list success")
	},
}

func init() {
	RootCmd.AddCommand(listCmd)
}

type Data struct {
	id int64
	metadata string
}

func executeList(args []string) error {
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

	if len(args) == 0 {
		sort.Slice(dataList, func(i, j int) bool {
			return dataList[i].id < dataList[j].id
		})
	
		for _, value := range dataList {
			fmt.Println(value.id, value.metadata)
		}
	} else {
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
			
				query := "SELECT data FROM secure_data WHERE id = ?"
				row := db.QueryRow(query, data.id)
				var encodedData string
				err = row.Scan(&encodedData)
				if err != nil {
					return err
				}
				encryptedDataFull, err := base64.StdEncoding.DecodeString(encodedData) 
				if err != nil {
					return fmt.Errorf("error while decode data from db: %w", err)
				}
			
				if len(encryptedDataFull) < 12 {
					return fmt.Errorf("corrupted data in db")
				}
			
				nonce := encryptedDataFull[0:12]
				encryptedData := encryptedDataFull[12:]
			
				key, err := deriveKey(pass)
				if err != nil {
					return err
				}
			
				decryptedData, err := decryptData(key, encryptedData, nonce)
				if err != nil {
					return err
				}
				fmt.Println(string(decryptedData))
			}
		}
	}

	return nil
}

func selectAll(pass []byte) ([]Data, error) {
	mail, err := getCurrentUser()
	if err != nil {
		return nil, fmt.Errorf("error while getting current user, try to use init command: %w", err)
	}
	if err := checkMasterPassword(mail, pass); err != nil {
		return nil, fmt.Errorf("wrong password: %w", err)
	}
	if err := sync(mail, pass); err != nil && err.Error() != "FULLY_SYNCED"{
		return nil, fmt.Errorf("error while sync: %w", err)
	}

	dbPath, err := getDBPath(mail)
	if err != nil {
		return nil, err
	}
	db, err := sql.Open("sqlite3", dbPath )
	if err != nil {
		return nil, err
	}
	defer db.Close()

	query := "SELECT id, metadata FROM secure_data WHERE is_active = true AND id != (SELECT MIN(id) FROM secure_data)"
	
	rows, err := db.Query(query, 25) 
	if err != nil {
		return nil, fmt.Errorf("db query error: %w", err)
	}
	defer rows.Close() 

	var dataList []Data
	for rows.Next() {
		var data Data
		err := rows.Scan(&data.id, &data.metadata)
		if err != nil {
			return nil, fmt.Errorf("error while scanning row from db: %w", err)
		}
		dataList = append(dataList, data)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows.Err() error: %w", err)
	}
	return dataList, err
}