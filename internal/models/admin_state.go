package models

type AdminState struct {
	UserID           int64
	CurrentState     string
	SelectedTypeID   int64
	DraftText        string
	DraftPhotoID     string
	DraftEntities    string
	EditingPostID    int64
	EditingTypeID    int64
	TempName         string
	TempEmoji        string
	TempPhotoID      string
	TempTemplate     string
	LastBotMessageID int
}
