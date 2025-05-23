/*
Copyright © 2023 LiuHailong
*/
package middleware

import (
	"fmt"

	"github.com/gin-gonic/gin"
)

func GinLogger() gin.HandlerFunc {
	return gin.LoggerWithFormatter(func(param gin.LogFormatterParams) string {

		statusColor := param.StatusCodeColor()
		methodColor := param.MethodColor()
		resetColor := param.ResetColor()

		return fmt.Sprintf("%s\t%s%s%s\t[%s]\t%s\t%s|%s %d %s|\t%s\t%s\n",
			param.TimeStamp.Format("2006-01-02 15:04:05.000000"),
			methodColor,
			param.Method,
			resetColor,
			param.ClientIP,
			param.Path,
			param.Request.Proto,
			statusColor,
			param.StatusCode,
			resetColor,
			param.Latency,
			param.ErrorMessage,
		)
	})
}
