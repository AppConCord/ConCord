// Package model defines the persistence entities shared by application services and GORM.
package model

// User is a registered Concord user. PasswordHash must never be serialized to an API response.
type User struct {
	ID           int64   `gorm:"column:id;primaryKey;autoIncrement:false"`
	Username     string  `gorm:"column:username;size:16;not null;uniqueIndex"`
	DisplayName  *string `gorm:"column:display_name;size:32"`
	PasswordHash string  `gorm:"column:password_hash;not null"`
	CreatedAt    int64   `gorm:"column:created_at;not null;autoCreateTime:milli"`
	UpdatedAt    int64   `gorm:"column:updated_at;not null;autoUpdateTime:milli"`
}

// Session is a revocable opaque bearer session. TokenHash stores SHA-256 output, never the token.
type Session struct {
	ID        int64  `gorm:"column:id;primaryKey;autoIncrement:false"`
	UserID    int64  `gorm:"column:user_id;not null;index"`
	TokenHash []byte `gorm:"column:token_hash;type:blob;size:32;not null;uniqueIndex"`
	ExpiresAt int64  `gorm:"column:expires_at;not null;index"`
	CreatedAt int64  `gorm:"column:created_at;not null;autoCreateTime:milli"`
	UpdatedAt int64  `gorm:"column:updated_at;not null;autoUpdateTime:milli"`
}

// Message is one entry in the global message feed.
type Message struct {
	ID        int64  `gorm:"column:id;primaryKey;autoIncrement:false"`
	UserID    int64  `gorm:"column:user_id;not null;index"`
	Content   string `gorm:"column:content;size:2000;not null"`
	CreatedAt int64  `gorm:"column:created_at;not null;autoCreateTime:milli"`
	UpdatedAt int64  `gorm:"column:updated_at;not null;autoUpdateTime:milli"`
	Author    User   `gorm:"foreignKey:UserID;references:ID"`
}
