package httpapi

import (
	"strconv"
	"time"

	"concord/backend/internal/model"
)

type userResponse struct {
	ID          string  `json:"id"`
	Username    string  `json:"username"`
	DisplayName *string `json:"display_name"`
	CreatedAt   string  `json:"created_at"`
	UpdatedAt   string  `json:"updated_at"`
}

type messageResponse struct {
	ID        string       `json:"id"`
	Content   string       `json:"content"`
	CreatedAt string       `json:"created_at"`
	UpdatedAt string       `json:"updated_at"`
	Author    userResponse `json:"author"`
}

func newUserResponse(user *model.User) userResponse {
	return userResponse{
		ID:          strconv.FormatInt(user.ID, 10),
		Username:    user.Username,
		DisplayName: user.DisplayName,
		CreatedAt:   formatTimestamp(user.CreatedAt),
		UpdatedAt:   formatTimestamp(user.UpdatedAt),
	}
}

func newMessageResponse(message *model.Message) messageResponse {
	return messageResponse{
		ID:        strconv.FormatInt(message.ID, 10),
		Content:   message.Content,
		CreatedAt: formatTimestamp(message.CreatedAt),
		UpdatedAt: formatTimestamp(message.UpdatedAt),
		Author:    newUserResponse(&message.Author),
	}
}

func formatTimestamp(milliseconds int64) string {
	return time.UnixMilli(milliseconds).UTC().Format("2006-01-02T15:04:05.000Z")
}
