package repository

import (
	"strings"

	"github.com/dujiao-next/internal/models"

	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// RechargeJobListFilter 兑换任务列表筛选。
type RechargeJobListFilter struct {
	Provider    string
	ProductType string
	Status      string
	CardKey     string
	JobID       string
	Page        int
	PageSize    int
}

// RechargeJobRepository 兑换任务本地记录仓库。
type RechargeJobRepository interface {
	List(filter RechargeJobListFilter) ([]models.RechargeJob, int64, error)
	GetByCardKey(cardKey string) (*models.RechargeJob, error)
	UpsertByCardKey(job *models.RechargeJob) error
}

type GormRechargeJobRepository struct {
	BaseRepository
}

func NewRechargeJobRepository(db *gorm.DB) *GormRechargeJobRepository {
	return &GormRechargeJobRepository{BaseRepository: BaseRepository{db: db}}
}

func (r *GormRechargeJobRepository) List(filter RechargeJobListFilter) ([]models.RechargeJob, int64, error) {
	query := r.db.Model(&models.RechargeJob{})
	if provider := strings.TrimSpace(filter.Provider); provider != "" {
		query = query.Where("provider = ?", provider)
	}
	if productType := strings.TrimSpace(filter.ProductType); productType != "" {
		query = query.Where("product_type = ?", productType)
	}
	if status := strings.TrimSpace(filter.Status); status != "" {
		query = query.Where("status = ?", status)
	}
	if cardKey := strings.TrimSpace(filter.CardKey); cardKey != "" {
		query = query.Where("LOWER(card_key) LIKE LOWER(?)", "%"+cardKey+"%")
	}
	if jobID := strings.TrimSpace(filter.JobID); jobID != "" {
		query = query.Where("LOWER(upstream_job_id) LIKE LOWER(?)", "%"+jobID+"%")
	}

	var total int64
	if err := query.Count(&total).Error; err != nil {
		return nil, 0, err
	}
	query = applyPagination(query, filter.Page, filter.PageSize)
	var rows []models.RechargeJob
	if err := query.Order("id desc").Find(&rows).Error; err != nil {
		return nil, 0, err
	}
	return rows, total, nil
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
