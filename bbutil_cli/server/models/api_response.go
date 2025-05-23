package models

const (
	SUCCESS_CODE = "1"
	FAIL_CODE    = "0"
)

type ApiResponse struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Success bool   `json:"success"`
	Data    any    `json:"data"`
}

func (a *ApiResponse) SuccessWithDataAndMessage(d any, m string) ApiResponse {
	return ApiResponse{
		SUCCESS_CODE, m, true, d,
	}
}

func (a *ApiResponse) SuccessWithData(d any) ApiResponse {
	return ApiResponse{
		SUCCESS_CODE, "", true, d,
	}
}

func (a *ApiResponse) FailWithMessage(m string) ApiResponse {
	return ApiResponse{
		FAIL_CODE, m, false, nil,
	}
}

func (a *ApiResponse) SuccessDefault() ApiResponse {
	return ApiResponse{
		SUCCESS_CODE,
		"",
		true,
		nil,
	}
}

func (a *ApiResponse) FailDefault() ApiResponse {
	return ApiResponse{
		FAIL_CODE,
		"服务器忙，请稍后重试",
		false,
		nil,
	}
}
