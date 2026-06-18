package repository

import (
	"strings"

	"github.com/dujiao-next/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RechargeJobRepository 兑换任务本地记录仓库。
type RechargeJobRepository interface {
	GetByCardKey(cardKey string) (*models.RechargeJob, error)
	UpsertByCardKey(job *models.RechargeJob) error
}

type GormRechargeJobRepository struct {
	BaseRepository
}

func NewRechargeJobRepository(db *gorm.DB) *GormRechargeJobRepository {
	return &GormRechargeJobRepository{BaseRepository: BaseRepository{db: db}}
}

func (r *GormRechargeJobRepository) GetByCardKey(cardKey string) (*models.RechargeJob, error) {
	cardKey = strings.TrimSpace(cardKey)
	if cardKey == "" {
		return nil, gorm.ErrRecordNotFound
	}
	var job models.RechargeJob
	if err := r.db.Where("card_key = ?", cardKey).First(&job).Error; err != nil {
		return nil, err
	}
	return &job, nil
}

func (r *GormRechargeJobRepository) UpsertByCardKey(job *models.RechargeJob) error {
	if job == nil {
		return nil
	}
	job.CardKey = strings.TrimSpace(job.CardKey)
	if job.CardKey == "" {
		return nil
	}
	return r.db.Clauses(clause.OnConflict{
		Columns: []clause.Column{{Name: "card_key"}},
		DoUpdates: clause.AssignmentColumns([]string{
			"provider",
			"product_type",
			"upstream_job_id",
			"status",
			"message",
			"activation_email",
			"plan_name",
			"raw_response",
			"submitted_at",
			"finished_at",
			"updated_at",
		}),
	}).Create(job).Error
}
