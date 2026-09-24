package model

import "time"

// User 用户/律师实体。
type User struct {
	ID           uint64 `gorm:"primaryKey" json:"id"`
	Username     string `gorm:"size:50;uniqueIndex;not null" json:"username"`
	PasswordHash string `gorm:"size:100;not null" json:"-"`
	RealName     string `gorm:"size:50;not null;default:''" json:"real_name"`
	Role         string `gorm:"size:20;not null;default:lawyer" json:"role"`
	LicenseNo    string `gorm:"size:50;not null;default:''" json:"license_no"`
	Email        string `gorm:"size:100;not null;default:''" json:"email"`
	Phone        string `gorm:"size:20;not null;default:''" json:"phone"`
	Avatar       string `gorm:"size:255;not null;default:''" json:"avatar"`
	// HourlyRate 律师当前每小时费率（元/小时），登记工时时作为默认费率快照；
	// 之后调整该值不影响已登记工时记录上的快照费率。
	HourlyRate float64   `gorm:"type:numeric(12,2);not null;default:0" json:"hourly_rate"`
	CreatedAt  time.Time `json:"created_at"`
}

// TableName 指定表名。
func (User) TableName() string { return "users" }
