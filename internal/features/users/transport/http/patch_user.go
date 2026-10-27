package users_transport_http

import (
	"fmt"
	"net/http"
	"strings"

	"github.com/Tutiu-Maksim/golang-todoapp/internal/core/domain"
	core_logger "github.com/Tutiu-Maksim/golang-todoapp/internal/core/logger"
	core_http_request "github.com/Tutiu-Maksim/golang-todoapp/internal/core/transport/http/request"
	core_http_response "github.com/Tutiu-Maksim/golang-todoapp/internal/core/transport/http/response"
	core_http_types "github.com/Tutiu-Maksim/golang-todoapp/internal/core/transport/http/types"
)

type PatchUserRequest struct {
	FullName    core_http_types.Nullable[string] `json:"full_name" swaggertype:"string" example:"Ivan Ivanov"`
	PhoneNumber core_http_types.Nullable[string] `json:"phone_number" swaggertype:"string" example:"+79998887766"`
}

func (r *PatchUserRequest) Validate() error {
	if r.FullName.Set {
		if r.FullName.Value == nil {
			return fmt.Errorf("'FullName' can't be NULL")
		}

		fullNameLen := len([]rune(*r.FullName.Value))
		if fullNameLen < 3 || fullNameLen > 100 {
			return fmt.Errorf("'FullName' must be between 3 and 100 symbol")
		}
	}

	if r.PhoneNumber.Set {
		if r.PhoneNumber.Value != nil {
			phoneNumberLem := len([]rune(*r.PhoneNumber.Value))
			if phoneNumberLem < 10 || phoneNumberLem > 15 {
				return fmt.Errorf("'PhoneNumber' must be between 10 and 15 symbol")
			}

			if !strings.HasPrefix(*r.PhoneNumber.Value, "+") {
				return fmt.Errorf("'PhoneNumber' must startswith '+' symbol")
			}
		}
	}

	return nil
}

type PatchUserResponce UserDTOResponse

// PatchUser godoc
// @Summary 	Обновить пользователя
// @Description Частично обновить данные пользователя по ID
// @Tags 		users
// @Accept 		json
// @Produce 	json
// @Param 		id 		path 	int 				true 	"ID пользователя" 			example(10)
// @Param 		request body 	PatchUserRequest 	true 	"PatchUser тело запроса"
// @Success 	200 	{object} PatchUserResponce 			"Успешно обновленный пользователь"
// @Failure 	400 	{object} core_http_response.ErrorResponse "Bad request"
// @Failure 	404 	{object} core_http_response.ErrorResponse "Not found"
// @Failure 	500 	{object} core_http_response.ErrorResponse "Internal server error"
// @Router 		/users/{id} [patch]
func (h *UsersHTTPHandler) PatchUser(rw http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	log := core_logger.FromContext(ctx)
	responseHandler := core_http_response.NewHTTPResponseHandler(log, rw)

	userID, err := core_http_request.GetIntPathValue(r, "id")
	if err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to get userID path value",
		)

		return
	}

	var request PatchUserRequest
	if err := core_http_request.DecodeAndValidateRequest(r, &request); err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to decode and validate HTTP request",
		)

		return
	}

	userPatch := userPatchFromRequest(request)

	userDomain, err := h.usersService.PatchUser(ctx, userID, userPatch)
	if err != nil {
		responseHandler.ErrorResponse(
			err,
			"failed to patch user",
		)

		return
	}

	response := PatchUserResponce(userDTOFromDomain(userDomain))

	responseHandler.JSONResponse(response, http.StatusOK)
}

func userPatchFromRequest(request PatchUserRequest) domain.UserPatch {
	return domain.NewUserPatch(
		request.FullName.ToDomain(),
		request.PhoneNumber.ToDomain(),
	)
}
