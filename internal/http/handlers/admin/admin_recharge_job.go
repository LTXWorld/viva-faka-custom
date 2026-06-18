package admin

import (
	"strings"

	"github.com/dujiao-next/internal/http/handlers/shared"
	"github.com/dujiao-next/internal/http/response"
	"github.com/dujiao-next/internal/repository"

	"github.com/gin-gonic/gin"
)

// GetRechargeJobs GET /api/v1/admin/recharge-jobs
// 后台查看 ChatGPT Plus 等兑换/直充任务本地记录。
func (h *Handler) GetRechargeJobs(c *gin.Context) {
	if h.RechargeJobRepo == nil {
		response.Error(c, response.CodeInternal, "recharge job repository not initialized")
		return
	}
	page, pageSize := shared.ParsePagination(c)
	rows, total, err := h.RechargeJobRepo.List(repository.RechargeJobListFilter{
		Provider:    strings.TrimSpace(c.Query("provider")),
		ProductType: strings.TrimSpace(c.Query("product_type")),
		Status:      strings.TrimSpace(c.Query("status")),
		CardKey:     strings.TrimSpace(c.Query("card_key")),
		JobID:       strings.TrimSpace(c.Query("job_id")),
		Page:        page,
		PageSize:    pageSize,
	})
	if err != nil {
		response.Error(c, response.CodeInternal, "查询兑换记录失败")
		return
	}
	response.SuccessWithPage(c, rows, response.BuildPagination(page, pageSize, total))
}
