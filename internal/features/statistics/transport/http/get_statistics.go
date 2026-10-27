package statistics_transport_http

import (
	"fmt"
	"net/http"
	"time"

	"github.com/Tutiu-Maksim/golang-todoapp/internal/core/domain"
	core_logger "github.com/Tutiu-Maksim/golang-todoapp/internal/core/logger"
	core_http_request "github.com/Tutiu-Maksim/golang-todoapp/internal/core/transport/http/request"
	core_http_response "github.com/Tutiu-Maksim/golang-todoapp/internal/core/transport/http/response"
)

type GetStatisticsResponse struct {
	TasksCreated               int      `json:"tasks_created" example:"15"`
	TasksCompleted             int      `json:"tasks_completed" example:"10"`
	TasksCompletedRate         *float64 `json:"tasks_completed_rate" example:"66.67"`
	TasksAverageCompletionTime *string  `json:"tasks_average_completion_time" example:"1h30m0s"`
}

// GetStatistics godoc
// @Summary 	Получить статистику
// @Description Получить статистику по задачам пользователя за указанный временной период
// @Tags 		statistics
// @Produce 	json
// @Param 		user_id query 	int 	false 	"ID пользователя" 								example(10)
// @Param 		from 	query 	string 	false 	"Дата начала периода (формат YYYY-MM-DD)" 		example(2026-01-01)
// @Param 		to 		query 	string 	false 	"Дата окончания периода (формат YYYY-MM-DD)" 	example(2026-12-31)
// @Success 	200 	{object} GetStatisticsResponse 			"Статистика по задачам"
// @Failure 	400 	{object} core_http_response.ErrorResponse 	"Bad request"
// @Failure 	500 	{object} core_http_response.ErrorResponse 	"Internal server error"
// @Router 		/statistics [get]
func (h *StatisticsHTTPHandler) GetStatistics(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := core_logger.FromContext(ctx)
	responseHandler := core_http_response.NewHTTPResponseHandler(log, rw)

	userID, from, to, err := getUserIDFromToQueryParams(r)
	if err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to get userID/from/to query params",
		)

		return
	}

	statistics, err := h.statisticsService.GetStatistics(ctx, userID, from, to)
	if err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to get statistics",
		)

		return
	}

	response := toDTOFromDomain(statistics)

	responseHandler.JSONResponse(response, http.StatusOK)
}

func toDTOFromDomain(statistics domain.Statistics) GetStatisticsResponse {
	var avgTime *string
	if statistics.TasksAverageCompletionTime != nil {
		duration := statistics.TasksAverageCompletionTime.String()
		avgTime = &duration
	}

	return GetStatisticsResponse{
		TasksCreated:               statistics.TasksCreated,
		TasksCompleted:             statistics.TasksCompleted,
		TasksCompletedRate:         statistics.TasksCompletedRate,
		TasksAverageCompletionTime: avgTime,
	}
}

func getUserIDFromToQueryParams(r *http.Request) (*int, *time.Time, *time.Time, error) {
	const (
		userIDQueryParam = "user_id"
		fromQueryParam   = "from"
		toQueryParam     = "to"
	)
	userID, err := core_http_request.GetIntQueryParam(r, "user_id")
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get 'user_id' query param: %w", err)
	}

	from, err := core_http_request.GetDateQueryParam(r, fromQueryParam)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get 'from' query param: %w", err)
	}

	to, err := core_http_request.GetDateQueryParam(r, toQueryParam)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("get 'to' query param: %w", err)
	}

	return userID, from, to, nil
}
