package commands

import (
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/user"
	"path/filepath"
	"time"

	_ "github.com/mattn/go-sqlite3"
	"github.com/spf13/cobra"
	"github.com/stepanov-ds/GophKeeper/internal/utils/structs"
	"golang.org/x/crypto/scrypt"
	"golang.org/x/term"
)

var serverURL = "http://localhost:8085"

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "initialize local store and register user",
	Run: func(cmd *cobra.Command, args []string) {
		if err := executeInit(); err != nil {
			fmt.Fprintf(os.Stderr, "init failed: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("init success")
	},
}

func init() {
	RootCmd.AddCommand(initCmd)
}

func executeInit() error {
	mail, masterPassword, err := promptUserCredentials()
	if err != nil {
		return fmt.Errorf("credentials error: %w", err)
	}

	exist, err := registerUser(mail)
	if err != nil {
		return fmt.Errorf("ошибка регистрации на сервере: %w", err)
	}

	if err := createAndSecureDB(mail); err != nil {
		return fmt.Errorf("ошибка создания локальной базы: %w", err)
	}

	token, err := login(mail)
	if err != nil {
		return fmt.Errorf("login error: %w", err)
	}
	if err := saveToken(token, masterPassword, mail); err != nil {
		return fmt.Errorf("error while saving token: %w", err)
	}
	if exist {
		for {
			err := sync(mail, masterPassword)
			if err != nil {
				if err.Error() == "FULLY_SYNCED" {
					break
				} else {
					return fmt.Errorf("error while syncing: %w", err)
				}
			}
			time.Sleep(1 * time.Second)
		}
		if err := checkMasterPassword(mail, masterPassword); err != nil {
			return nil
		}
	} else {
		if err := createMasterPasswordCheck(mail, masterPassword); err != nil {
			return fmt.Errorf("error while createMasterPasswordCheck: %w", err)
		}
	}

	err = setCurrentUser(mail)
	if err != nil {
		return fmt.Errorf("error while setting current user: %w", err)
	}

	return nil
}

func promptUserCredentials() (string, []byte, error) {
	fmt.Print("enter your mail: ")
	var mail string
	if _, err := fmt.Scanln(&mail); err != nil {
		return "", nil, err
	}

	fmt.Print("enter your master-password: ")
	pass1, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", nil, err
	}
	fmt.Println()

	fmt.Print("enter your master-password again: ")
	pass2, err := term.ReadPassword(int(os.Stdin.Fd()))
	if err != nil {
		return "", nil, err
	}
	fmt.Println()

	if string(pass1) != string(pass2) {
		return "", nil, fmt.Errorf("master-passwords does not match")
	}

	return mail, pass1, nil
}

func registerUser(mail string) (bool, error) {
	var bodyJSON struct {
		Mail string `json:"mail"`
	}
	bodyJSON.Mail = mail

	jsonData, _ := json.Marshal(bodyJSON)

	resp, err := http.Post(serverURL+"/register", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return false, fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusConflict {
			return true, nil
		}
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return false, err
		}
		return false, fmt.Errorf("bad responce: %d, responce body: %s", resp.StatusCode, string(body))
	}

	return false, nil
}

func getStoragePath() (string, error) {
	currentUser, err := user.Current()
	if err != nil {
		return "", err
	}

	appDir := filepath.Join(currentUser.HomeDir, ".pman")
	if err := os.MkdirAll(appDir, 0700); err != nil {
		return "", err
	}
	return appDir, nil
}

func createAndSecureDB(mail string) error {
	dbPath, err := getDBPath(mail)
	if err != nil {
		return err
	}

	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS secure_data (
    	id INTEGER PRIMARY KEY,
    	data TEXT NOT NULL,
    	metadata TEXT NOT NULL,
    	history_id INTEGER,
    	is_active BOOLEAN
	);
	CREATE INDEX IF NOT EXISTS idx_secure_data_metadata
	ON secure_data (metadata);`

	if _, err := db.Exec(createTableSQL); err != nil {
		return err
	}

	return nil
}

func login(mail string) (string, error) {
	var bodyJSON struct {
		Mail     string `json:"mail"`
		Password string `json:"password"`
	}
	bodyJSON.Mail = mail

	jsonData, _ := json.Marshal(bodyJSON)

	resp, err := http.Post(serverURL+"/loginGet", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("bad responce: %d, responce body: %s", resp.StatusCode, string(body))
	}

	fmt.Print("confirmation code sent to your email, please enter: ")
	var code string
	if _, err := fmt.Scanln(&code); err != nil {
		return "", err
	}
	bodyJSON.Password = code

	jsonData, _ = json.Marshal(bodyJSON)

	resp, err = http.Post(serverURL+"/loginPost", "application/json", bytes.NewBuffer(jsonData))
	if err != nil {
		return "", fmt.Errorf("request error: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return "", err
		}
		return "", fmt.Errorf("bad responce: %d, responce body: %s", resp.StatusCode, string(body))
	}
	var token string
	for _, cookie := range resp.Cookies() {
		if cookie.Name == "Authorization" {
			token = cookie.Value
		}
	}
	if token == "" {
		return "", fmt.Errorf("no authorization cookie in response: %w", err)
	}

	return token, nil
}

const MasterKeySalt = "pman-salt-v1"

func deriveKey(masterPassword []byte) ([]byte, error) {
	key, err := scrypt.Key(
		masterPassword,
		[]byte(MasterKeySalt),
		32768, 8, 1, 32)

	if err != nil {
		return nil, err
	}
	return key, nil 
}

func encryptData(key, plaintext []byte) (ciphertext []byte, nonce []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}

	nonce = make([]byte, aesgcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}

	ciphertext = aesgcm.Seal(nil, nonce, plaintext, nil)

	return ciphertext, nonce, nil
}

func decryptData(key, ciphertext, nonce []byte) ([]byte, error) {

	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}

	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}

	plaintext, err := aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}

	return plaintext, nil
}

func saveToken(token string, masterPassword []byte, mail string) error {
	key, err := deriveKey(masterPassword)
	if err != nil {
		return err
	}
	encryptedData, nonce, err := encryptData(key, []byte(token))
	if err != nil {
		return err
	}

	fp, err := getTokenPath(mail)
	if err != nil {
		return err
	}

	var buffer bytes.Buffer

	buffer.Write(nonce)
	buffer.Write(encryptedData)

	finalData := buffer.Bytes()

	err = os.WriteFile(fp, finalData, 0600)
	if err != nil {
		return err
	}

	return nil
}

func createMasterPasswordCheck(mail string, masterPassword []byte) error {
	data := []byte("MASTER_PASSWORD_CHECK")
	metadata := `{"type": "MASTER_PASSWORD_CHECK"}`
	return addSecureData(mail, masterPassword, data, json.RawMessage(metadata))
}

func sync(mail string, masterPassword []byte) error {
	var bodyJSON struct {
		Last  int64 `json:"lastHistoryID"`
		Limit int   `json:"limit"`
	}

	dbPath, err := getDBPath(mail)
	if err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", dbPath)
	if err != nil {
		return err
	}
	defer db.Close()

	query := "SELECT COALESCE(MAX(history_id), 0) FROM secure_data"
	row := db.QueryRow(query)
	var lastHistoryID int64
	err = row.Scan(&lastHistoryID)
	if err != nil {
		if err == sql.ErrNoRows {
			lastHistoryID = 0
		} else {
			return err
		}
	}
	bodyJSON.Last = lastHistoryID
	bodyJSON.Limit = 10
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

	jsonData, _ := json.Marshal(bodyJSON)
	req, err := http.NewRequest("POST", serverURL+"/sync", bytes.NewBuffer(jsonData))
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

	if len(respJSON.SecureData) > 0 {
		for _, value := range respJSON.SecureData {
			insertQuery := `INSERT OR REPLACE INTO secure_data (id, data, metadata, history_id, is_active)
							VALUES (?, ?, ?, ?, ?)`
			if _, err := db.Exec(insertQuery, value.ID, value.Data, value.Metadata, value.HistoryID, value.IsActive); err != nil {
				return err
			}
		}
		if respJSON.FullySynced {
			return fmt.Errorf("FULLY_SYNCED")
		}
	} else {
		return fmt.Errorf("FULLY_SYNCED")
	}

	return nil
}

func checkMasterPassword(mail string, masterPassword []byte) error {
	dbPath, err := getDBPath(mail)
	if err != nil {
		return err
	}
	db, err := sql.Open("sqlite3", dbPath )
	if err != nil {
		return err
	}
	defer db.Close()

	query := "SELECT data FROM secure_data where id in (SELECT MIN(id) FROM secure_data)"
	row := db.QueryRow(query)
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

	key, err := deriveKey(masterPassword)
	if err != nil {
		return err
	}

	decryptedData, err := decryptData(key, encryptedData, nonce)
	if err != nil {
		return err
	}
	if string(decryptedData) != "MASTER_PASSWORD_CHECK" {
		return fmt.Errorf("master password incorrect")
	}

	return nil
}

func setCurrentUser(mail string) error {
	path, err := getCurrentUserPath()
	if err != nil {
		return err
	}

	var buffer bytes.Buffer

	buffer.Write([]byte(mail))

	finalData := buffer.Bytes()

	err = os.WriteFile(path, finalData, 0600)
	if err != nil {
		return err
	}

	return nil
}

func getDBPath(mail string) (string, error) {
    storagePath, err := getStoragePath()
    if err != nil {
        return "", err
    }
    return filepath.Join(storagePath, mail + "_vault.db"), nil
}

func getTokenPath(mail string) (string, error) {
    storagePath, err := getStoragePath()
    if err != nil {
        return "", err
    }
    return filepath.Join(storagePath, mail + "_token"), nil
}

func getCurrentUserPath() (string, error) {
    storagePath, err := getStoragePath()
    if err != nil {
        return "", err
    }
    return filepath.Join(storagePath, "current"), nil
}