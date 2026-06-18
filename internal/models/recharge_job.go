package models

import (
	"time"

	"gorm.io/gorm"
)

const (
	RechargeProviderLyxazy     = "lyxazy"
	RechargeProductChatGPTPlus = "chatgpt_plus"
	RechargeStatusSubmitted    = "submitted"
	RechargeStatusQueued       = "queued"
	RechargeStatusRunning      = "running"
	RechargeStatusSuccess      = "success"
	RechargeStatusFailed       = "failed"
	RechargeStatusCancelled    = "cancelled"
)

// RechargeJob 兑换/直充任务本地记录。
// 说明：不保存用户 AccessToken/session 等敏感提交内容，仅保存上游任务与状态结果。
type RechargeJob struct {
	ID              uint           `gorm:"primarykey" json:"id"`
	Provider        string         `gorm:"type:varchar(50);not null;index:idx_recharge_provider_product" json:"provider"`
	ProductType     string         `gorm:"type:varchar(50);not null;index:idx_recharge_provider_product" json:"product_type"`
	CardKey         string         `gorm:"type:varchar(255);not null;uniqueIndex" json:"card_key"`
	UpstreamJobID   string         `gorm:"type:varchar(100);index" json:"upstream_job_id"`
	Status          string         `gorm:"type:varchar(30);not null;index" json:"status"`
	Message         string         `gorm:"type:text" json:"message"`
	ActivationEmail string         `gorm:"type:varchar(255)" json:"activation_email"`
	PlanName        string         `gorm:"type:varchar(100)" json:"plan_name"`
	RawResponseJSON JSON           `gorm:"column:raw_response;type:json" json:"raw_response"`
	SubmittedAt     *time.Time     `gorm:"index" json:"submitted_at"`
	FinishedAt      *time.Time     `gorm:"index" json:"finished_at"`
	CreatedAt       time.Time      `gorm:"index" json:"created_at"`
	UpdatedAt       time.Time      `gorm:"index" json:"updated_at"`
	DeletedAt       gorm.DeletedAt `gorm:"index" json:"-"`
}

func (RechargeJob) TableName() string {
	return "recharge_jobs"
}
