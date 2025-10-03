package structs

type SecureData struct {
	ID int64 `json:"ID"`
	Data string `json:"data"`
	Metadata string `json:"metadata"`
	IsActive bool `json:"isActive"`
	HistoryID int64 `json:"historyID"`
}