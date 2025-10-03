package commands

import (
	"bufio"
	"bytes"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/stepanov-ds/GophKeeper/internal/utils/structs"
	"golang.org/x/term"
)

var addCmd = &cobra.Command{
	Use:   "add",
	Short: "add new secure data",
	Run: func(cmd *cobra.Command, args []string) {
		if err := executeAdd(); err != nil {
			fmt.Fprintf(os.Stderr, "add failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("add success")
	},
}

func init() {
	RootCmd.AddCommand(addCmd)
}

func executeAdd() error {
	mail, err := getCurrentUser()
	if err != nil {
		return fmt.Errorf("error while getting current user, try to use init command: %w", err)
	}
	
	fmt.Print("enter your master-password: ")
	pass, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return err
	}
	fmt.Println()

	if err := checkMasterPassword(mail, pass); err != nil {
		return fmt.Errorf("wrong password: %w", err)
	}
	fmt.Print("enter name of service you want to add: ")
	var service string
	if _, err := fmt.Scanln(&service); err != nil {
		return err
	}

	
	metadata := make(map[string]string)
	reader := bufio.NewReader(os.Stdin) 
	fmt.Println("--- Enter structure metadata ---")
	fmt.Println("Enter key (example: 'login').")
	fmt.Println("Enter value (example: 'user123').")
	fmt.Println("Repeat if needed")
	fmt.Println("Enter empty string for continue")
	
	for {
		fmt.Print("Key (or Enter for continue): ")
		keyInput, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("error while reading key: %w", err)
		}
		key := strings.TrimSpace(keyInput)

		if key == "" {
			break
		}
		
		fmt.Printf("Value for '%s': ", key)
		valueInput, err := reader.ReadString('\n')
		if err != nil {
			return fmt.Errorf("error while reading value: %w", err)
		}
		value := strings.TrimSpace(valueInput)

		metadata[key] = value
		fmt.Printf("Saved: %s -> %s\n", key, value)
	}

	fmt.Println("--- Enter secure data ---")
	fmt.Println("Is yoursecure data multiline? (Y/N): ")
    typeInput, _ := reader.ReadString('\n')
    isMultiLine := strings.TrimSpace(strings.ToLower(typeInput)) == "y"
    
    var buffer []byte
    if isMultiLine {
        fmt.Println("Enter multiline value")
        buffer, err = readMultiLineValue(reader)
		if err != nil {
			return  err
		}
    } else {
        fmt.Printf("Secure data: ",)
        valueInput, err := reader.ReadString('\n')
        if err != nil {
            return fmt.Errorf("error while reading value: %w", err)
        }
        buffer = []byte(valueInput) 
    }
	jsonData, err := json.Marshal(metadata)
	if err := addSecureData(mail, pass, buffer, jsonData); err != nil {
		return fmt.Errorf("error while adding secure data: %w", err)
	}

	return nil
}

func getCurrentUser() (string, error) {
	filePath, err := getCurrentUserPath()
	if err != nil {
		return "", err
	}

	fileData, err := os.ReadFile(filePath)
	return string(fileData), err
}

func readMultiLineValue(reader *bufio.Reader) ([]byte, error) {
    var buffer bytes.Buffer
    fmt.Println("(Enter $$$$$ on separate line to finish multiline)")

    for {
        line, err := reader.ReadString('\n')
        if err != nil && err != io.EOF {
            return nil, err
        }
		lineBytes := []byte(line) 

		trimmedLine := strings.TrimSpace(line)
        if trimmedLine == "$$$$$" {
            break
        }
        
        buffer.Write(lineBytes)
        
        if err == io.EOF {
            break
        }
    }

    return buffer.Bytes(), nil
}

func addSecureData(mail string, masterPassword []byte, data []byte, metadata json.RawMessage) error {
	var bodyJSON struct {
		Type     string          `json:"type"`
		Data     string          `json:"data,omitempty"`
		Metadata json.RawMessage `json:"metadata,omitempty"`
	}
	bodyJSON.Type = "ADD"

	filePath, err := getTokenPath(mail)
	if err != nil {
		return err
	}

	fileData, err := os.ReadFile(filePath)
	nonce := fileData[0:12]
	encryptedData := fileData[12:]

	key, err := deriveKey(masterPassword)
	if err != nil {
		return err
	}
	token, err := decryptData(key, encryptedData, nonce)
	if err != nil {
		return err
	}

	encryptedData, nonce, err = encryptData(key, []byte(data))
	if err != nil {
		return err
	}

	bodyJSON.Data = base64.StdEncoding.EncodeToString([]byte(string(nonce) + string(encryptedData)))
	bodyJSON.Metadata = metadata

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

	dbPath, err := getDBPath(mail)
	if err != nil {
		return err
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	addSQL := `
	INSERT INTO secure_data (id, data, metadata, history_id, is_active)
	VALUES (?, ?, ?, ?, 1)`

	if _, err := db.Exec(addSQL, respJSON.SecureDataID, bodyJSON.Data, metadata, respJSON.HistoryID); err != nil {
		return err
	}
	return nil
}