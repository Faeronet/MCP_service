package modules

// FromNoteItem mirrors a row from angels-web note export (time + validation name + display name).
type FromNoteItem struct {
	Validation string `json:"validation"`
	Name       string `json:"name"`
	Time       string `json:"time"`
	Message    string `json:"message,omitempty"`
}

// FromNoteRequest is POST /schedule/from-note body.
type FromNoteRequest struct {
	TelegramUsername string         `json:"telegram_username"`
	TelegramID       int64          `json:"telegram_id,omitempty"`
	Items            []FromNoteItem `json:"items"`
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
	NameRU  string
	Valid   string
	TimeHH  int
	TimeMM  int
	Message string
}
