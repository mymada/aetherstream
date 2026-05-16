package api

import (
	"net/http"
	"time"

	"github.com/devuser/aetherstream/pkg/auth"
	"github.com/devuser/aetherstream/pkg/db"
	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
)

// ProfileRoutes handles user profile management (Netflix-style sub-profiles).
type ProfileRoutes struct {
	db   *db.DB
	auth *auth.Service
}

// NewProfileRoutes creates profile routes.
func NewProfileRoutes(database *db.DB, authSvc *auth.Service) *ProfileRoutes {
	return &ProfileRoutes{db: database, auth: authSvc}
}

// RegisterRoutes registers profile API routes.
func (pr *ProfileRoutes) RegisterRoutes(e *echo.Echo, authMiddleware echo.MiddlewareFunc) {
	api := e.Group("/api/profiles")
	api.Use(authMiddleware)

	api.GET("", pr.handleListProfiles)
	api.POST("", pr.handleCreateProfile)
	api.GET("/:id", pr.handleGetProfile)
	api.PUT("/:id", pr.handleUpdateProfile)
	api.DELETE("/:id", pr.handleDeleteProfile)
	api.POST("/:id/switch", pr.handleSwitchProfile)

	// Preferences
	api.GET("/:id/preferences", pr.handleGetPreferences)
	api.PUT("/:id/preferences", pr.handleUpdatePreferences)
}

func (pr *ProfileRoutes) handleListProfiles(c echo.Context) error {
	// Get account ID from context (set by auth middleware)
	accountID := c.Get("account_id").(string)
	profiles, err := pr.db.GetProfilesByAccount(accountID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, profiles)
}

func (pr *ProfileRoutes) handleCreateProfile(c echo.Context) error {
	accountID := c.Get("account_id").(string)

	// Check profile limit
	count, err := pr.db.CountProfiles(accountID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	if count >= 5 {
		return echo.NewHTTPError(http.StatusForbidden, "maximum number of profiles reached (5)")
	}

	var req struct {
		Name   string `json:"name"`
		Avatar string `json:"avatar"`
		PIN    string `json:"pin,omitempty"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}
	if req.Name == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "name is required")
	}

	profile := &db.UserProfile{
		ID:           uuid.New().String(),
		AccountID:    accountID,
		Name:         req.Name,
		Avatar:       req.Avatar,
		PIN:          req.PIN,
		Theme:        "dark",
		AutoplayNext: true,
		ShowBackdrop: true,
		AudioLanguage: "fra",
		SubtitleMode:  "default",
		CreatedAt:     time.Now(),
	}

	if err := pr.db.CreateProfile(profile); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusCreated, profile)
}

func (pr *ProfileRoutes) handleGetProfile(c echo.Context) error {
	profile, err := pr.db.GetProfileByID(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "profile not found")
	}
	return c.JSON(http.StatusOK, profile)
}

func (pr *ProfileRoutes) handleUpdateProfile(c echo.Context) error {
	var req struct {
		Name                string `json:"name"`
		Avatar              string `json:"avatar"`
		AudioLanguage       string `json:"audioLanguage"`
		SubtitleLanguage    string `json:"subtitleLanguage"`
		SubtitleMode        string `json:"subtitleMode"`
		AutoplayNext        bool   `json:"autoplayNext"`
		MaxParentalRating   int    `json:"maxParentalRating"`
		BlockedTags         string `json:"blockedTags"`
		Theme               string `json:"theme"`
		ShowBackdrop        bool   `json:"showBackdrop"`
		PreferredCastDevice string `json:"preferredCastDevice"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	profile, err := pr.db.GetProfileByID(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "profile not found")
	}

	profile.Name = req.Name
	profile.Avatar = req.Avatar
	profile.AudioLanguage = req.AudioLanguage
	profile.SubtitleLanguage = req.SubtitleLanguage
	profile.SubtitleMode = req.SubtitleMode
	profile.AutoplayNext = req.AutoplayNext
	profile.MaxParentalRating = req.MaxParentalRating
	profile.BlockedTags = req.BlockedTags
	profile.Theme = req.Theme
	profile.ShowBackdrop = req.ShowBackdrop
	profile.PreferredCastDevice = req.PreferredCastDevice

	if err := pr.db.UpdateProfile(profile); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, profile)
}

func (pr *ProfileRoutes) handleDeleteProfile(c echo.Context) error {
	if err := pr.db.DeleteProfile(c.Param("id")); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.NoContent(http.StatusNoContent)
}

func (pr *ProfileRoutes) handleSwitchProfile(c echo.Context) error {
	profileID := c.Param("id")
	claims := auth.GetUser(c)
	if claims == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "not authenticated")
	}

	profile, err := pr.db.GetProfileByID(profileID)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "profile not found")
	}
	if profile.AccountID != claims.UserID {
		return echo.NewHTTPError(http.StatusForbidden, "profile not owned by account")
	}

	profile.LastActivityAt = time.Now()
	pr.db.UpdateProfile(profile)

	token, err := pr.auth.GenerateTokenWithProfile(claims.UserID, claims.Username, claims.Role, profileID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to generate token")
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"profile": profile,
		"token":   token,
	})
}

func (pr *ProfileRoutes) handleGetPreferences(c echo.Context) error {
	profile, err := pr.db.GetProfileByID(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "profile not found")
	}
	return c.JSON(http.StatusOK, map[string]interface{}{
		"audioLanguage":       profile.AudioLanguage,
		"subtitleLanguage":    profile.SubtitleLanguage,
		"subtitleMode":        profile.SubtitleMode,
		"autoplayNext":        profile.AutoplayNext,
		"theme":               profile.Theme,
		"showBackdrop":        profile.ShowBackdrop,
		"preferredCastDevice": profile.PreferredCastDevice,
	})
}

func (pr *ProfileRoutes) handleUpdatePreferences(c echo.Context) error {
	var req struct {
		AudioLanguage       string `json:"audioLanguage"`
		SubtitleLanguage    string `json:"subtitleLanguage"`
		SubtitleMode        string `json:"subtitleMode"`
		AutoplayNext        bool   `json:"autoplayNext"`
		Theme               string `json:"theme"`
		ShowBackdrop        bool   `json:"showBackdrop"`
		PreferredCastDevice string `json:"preferredCastDevice"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid request")
	}

	profile, err := pr.db.GetProfileByID(c.Param("id"))
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, "profile not found")
	}

	profile.AudioLanguage = req.AudioLanguage
	profile.SubtitleLanguage = req.SubtitleLanguage
	profile.SubtitleMode = req.SubtitleMode
	profile.AutoplayNext = req.AutoplayNext
	profile.Theme = req.Theme
	profile.ShowBackdrop = req.ShowBackdrop
	profile.PreferredCastDevice = req.PreferredCastDevice

	if err := pr.db.UpdateProfile(profile); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, err.Error())
	}
	return c.JSON(http.StatusOK, profile)
}
