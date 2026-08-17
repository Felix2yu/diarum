package api

import (
	"net/http"

	"github.com/labstack/echo/v5"
)

func badRequest(message string, err error) error {
	if err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, message+": "+err.Error())
	}
	return echo.NewHTTPError(http.StatusBadRequest, message)
}

func unauthorized(message string) error {
	return echo.NewHTTPError(http.StatusUnauthorized, message)
}

func forbidden(message string) error {
	return echo.NewHTTPError(http.StatusForbidden, message)
}

func notFound(message string) error {
	return echo.NewHTTPError(http.StatusNotFound, message)
}

// serviceUnavailable returns a 503 with a structured body so the frontend can
// reliably distinguish "service not configured" from other client errors.
func serviceUnavailable(message string, err error) error {
	status := http.StatusServiceUnavailable
	if err != nil {
		return echo.NewHTTPError(status, message+": "+err.Error())
	}
	return echo.NewHTTPError(status, message)
}

func serverError(message string, err error) error {
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, message+": "+err.Error())
	}
	return echo.NewHTTPError(http.StatusInternalServerError, message)
}

// maskedSecret 是密钥掩码值，用于避免在 API 响应中回读明文密钥
const maskedSecret = "********"

// maskSecret 对已设置的密钥返回掩码，未设置（空）时保持空
func maskSecret(value string) string {
	if value == "" {
		return ""
	}
	return maskedSecret
}

// isSecretPlaceholder 判断提交的值是否为"不修改"占位符（仅掩码值）。
// 空串表示用户显式清除密钥，应作为新值提交；掩码值来自前端回传，需保留原密钥。
func isSecretPlaceholder(value string) bool {
	return value == maskedSecret
}
