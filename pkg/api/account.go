package api

import (
	"net/http"

	"github.com/devuser/aetherstream/pkg/db"
	"github.com/labstack/echo/v4"
	"golang.org/x/crypto/bcrypt"
)

// AccountRoutes handles account self-management (each user manages their own account).
type AccountRoutes struct {
	db *db.DB
}

// NewAccountRoutes creates account self-management routes.
func NewAccountRoutes(database *db.DB) *AccountRoutes {
	return &AccountRoutes{db: database}
}

// RegisterRoutes registers account self-management routes.
func (ar *AccountRoutes) RegisterRoutes(e *echo.Echo, authMiddleware echo.MiddlewareFunc) {
	api := e.Group("/api/account")
	api.Use(authMiddleware)

	api.GET("/me", ar.handleGetMyAccount)
	api.PUT("/me", ar.handleUpdateMyAccount)
	api.POST("/me/password", ar.handleChangePassword)
	api.GET("/me/devices", ar.handleGetMyDevices)
	api.DELETE("/me/devices/:id", ar.handleRevokeDevice)
	api.GET("/me/activity", ar.handleGetMyActivity)
}

func (ar *AccountRoutes) handleGetMyAccount(c echo.Context) error {
	accountID, ok := c.Get("account_id").(string)
	if !ok || accountID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	account, err := ar.db.GetAccountByID(accountID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "account not found")
	}

	return c.JSON(http.StatusOK, map[string]interface{}{
		"id":          account.ID,
		"email":       account.Email,
		"role":        account.Role,
		"maxProfiles": account.MaxProfiles,
		"dataQuotaMB": account.DataQuotaMB,
		"dataUsedMB":  account.DataUsedMB,
		"createdAt":   account.CreatedAt,
	})
}

func (ar *AccountRoutes) handleUpdateMyAccount(c echo.Context) error {
	accountID, ok := c.Get("account_id").(string)
	if !ok || accountID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	var req struct {
		Email string `json:"email"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	if req.Email != "" {
		if err := ar.db.UpdateAccountEmail(accountID, req.Email); err != nil {
			return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
		}
	}

	return c.NoContent(http.StatusNoContent)
}

func (ar *AccountRoutes) handleChangePassword(c echo.Context) error {
	accountID, ok := c.Get("account_id").(string)
	if !ok || accountID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	var req struct {
		CurrentPassword string `json:"currentPassword"`
		NewPassword     string `json:"newPassword"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	if req.CurrentPassword == "" || req.NewPassword == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "current and new password required")
	}

	account, err := ar.db.GetAccountByID(accountID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "account not found")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(account.PasswordHash), []byte(req.CurrentPassword)); err != nil {
		return echo.NewHTTPError(http.StatusForbidden, "current password incorrect")
	}

	hashed, err := bcrypt.GenerateFromPassword([]byte(req.NewPassword), bcrypt.DefaultCost)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to hash password")
	}

	if err := ar.db.UpdateAccountPassword(accountID, string(hashed)); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.NoContent(http.StatusNoContent)
}

func (ar *AccountRoutes) handleGetMyDevices(c echo.Context) error {
	accountID, ok := c.Get("account_id").(string)
	if !ok || accountID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	devices, err := ar.db.GetDevicesByAccount(accountID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, devices)
}

func (ar *AccountRoutes) handleRevokeDevice(c echo.Context) error {
	accountID, ok := c.Get("account_id").(string)
	if !ok || accountID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	deviceID := c.Param("id")
	if err := ar.db.RevokeDevice(accountID, deviceID); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

func (ar *AccountRoutes) handleGetMyActivity(c echo.Context) error {
	accountID, ok := c.Get("account_id").(string)
	if !ok || accountID == "" {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	activity, err := ar.db.GetActivityByAccount(accountID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}

	return c.JSON(http.StatusOK, activity)
}
