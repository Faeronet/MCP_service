package modules

// FromNoteItem mirrors a row from angels-web note export (time + validation name + display name).
type FromNoteItem struct {
	NoteItemID string `json:"note_item_id,omitempty"`
	Validation string `json:"validation"`
	Name       string `json:"name"`
	KeyName    string `json:"keyName,omitempty"`
	Time       string `json:"time"`
	Part       string `json:"part,omitempty"`
	Message    string `json:"message,omitempty"`
	NotifyDaily bool  `json:"notify_daily,omitempty"`
}

// FromNoteRequest is POST /schedule/from-note body.
type FromNoteRequest struct {
	TelegramUsername string         `json:"telegram_username"`
	TelegramID       int64          `json:"telegram_id,omitempty"`
	Items            []FromNoteItem `json:"items"`
	Sync             bool           `json:"sync,omitempty"`
}

// FromNoteResponse reports scheduling outcome.
type FromNoteResponse struct {
	Accepted           bool     `json:"accepted"`
	ScheduledCount     int      `json:"scheduled_count"`
	GroupedAngelsCount int      `json:"grouped_angels_count"`
	Errors             []string `json:"errors,omitempty"`
}

type groupedAngel struct {
	Key     string
	NoteItemID string
	NameRU  string
	Valid   string
	KeyName string
	TimeRaw string
	TimeHH  int
	TimeMM  int
	Part    string
	Message string
	NotifyDaily bool
}
