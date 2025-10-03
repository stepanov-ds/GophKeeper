package structs

type Response struct {
	Message string `json:"message,omitempty"`
	Error   string `json:"error,omitempty"`
    SecureDataID int64 `json:"SecureDataID,omitempty"`
    HistoryID int64 `json:"historyID,omitempty"`
	SecureData []SecureData `json:"secureData,omitempty"`
	FullySynced bool `json:"fullySynced,omitempty"`
}
