package playback

import (
	"net/http"
	"time"

	"github.com/labstack/echo/v4"
	"github.com/devuser/aetherstream/pkg/auth"
)

// EventPublisher is called after each successful command to forward events in real time.
type EventPublisher func(event PlaybackEvent)

// Handler holds dependencies for playback HTTP handlers.
type Handler struct {
	store     Store
	publisher EventPublisher
}

// NewHandler creates a playback API handler.
func NewHandler(store Store, publisher EventPublisher) *Handler {
	return &Handler{
		store:     store,
		publisher: publisher,
	}
}

// RegisterRoutes mounts playback endpoints under the provided Echo group.
// The group should already have auth middleware applied.
func (h *Handler) RegisterRoutes(g *echo.Group) {
	g.POST("/playback/start", h.handleStart)
	g.GET("/playback", h.handleList)
	g.GET("/playback/:id", h.handleGet)
	g.POST("/playback/:id/play", h.handlePlay)
	g.POST("/playback/:id/pause", h.handlePause)
	g.POST("/playback/:id/seek", h.handleSeek)
	g.POST("/playback/:id/volume", h.handleVolume)
	g.POST("/playback/:id/stop", h.handleStop)
}

func (h *Handler) handleStart(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	var req struct {
		ItemID   string `json:"item_id"`
		DeviceID string `json:"device_id"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body: "+err.Error())
	}
	if req.ItemID == "" || req.DeviceID == "" {
		return echo.NewHTTPError(http.StatusBadRequest, "item_id and device_id are required")
	}

	session, err := h.store.CreateSession(user.UserID, req.ItemID, req.DeviceID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to create session: "+err.Error())
	}

	h.publish(session.ID, CommandPlay, session.Position, session.Volume)
	return c.JSON(http.StatusCreated, session)
}

func (h *Handler) handleList(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	sessions, err := h.store.ListActiveSessions(user.UserID)
	if err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to list sessions: "+err.Error())
	}
	return c.JSON(http.StatusOK, sessions)
}

func (h *Handler) handleGet(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	id := c.Param("id")
	session, err := h.store.GetSession(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	if session.UserID != user.UserID {
		return echo.NewHTTPError(http.StatusForbidden, "access denied")
	}
	return c.JSON(http.StatusOK, session)
}

func (h *Handler) handlePlay(c echo.Context) error {
	return h.updateState(c, StatePlaying, CommandPlay)
}

func (h *Handler) handlePause(c echo.Context) error {
	return h.updateState(c, StatePaused, CommandPause)
}

func (h *Handler) handleStop(c echo.Context) error {
	return h.updateState(c, StateStopped, CommandStop)
}

func (h *Handler) handleSeek(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	id := c.Param("id")
	var req struct {
		Position int64 `json:"position"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body: "+err.Error())
	}
	if req.Position < 0 {
		return echo.NewHTTPError(http.StatusBadRequest, "position must be >= 0")
	}

	session, err := h.store.GetSession(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	if session.UserID != user.UserID {
		return echo.NewHTTPError(http.StatusForbidden, "access denied")
	}

	session.State = StatePlaying
	session.Position = req.Position
	if err := h.store.UpdateSession(session); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update session: "+err.Error())
	}

	h.publish(session.ID, CommandSeek, session.Position, session.Volume)
	return c.JSON(http.StatusOK, session)
}

func (h *Handler) handleVolume(c echo.Context) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	id := c.Param("id")
	var req struct {
		Volume int `json:"volume"`
	}
	if err := c.Bind(&req); err != nil {
		return echo.NewHTTPError(http.StatusBadRequest, "invalid body: "+err.Error())
	}
	if req.Volume < 0 || req.Volume > 100 {
		return echo.NewHTTPError(http.StatusBadRequest, "volume must be between 0 and 100")
	}

	session, err := h.store.GetSession(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	if session.UserID != user.UserID {
		return echo.NewHTTPError(http.StatusForbidden, "access denied")
	}

	session.Volume = req.Volume
	if err := h.store.UpdateSession(session); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update session: "+err.Error())
	}

	h.publish(session.ID, CommandVolume, session.Position, session.Volume)
	return c.JSON(http.StatusOK, session)
}

func (h *Handler) updateState(c echo.Context, state PlaybackState, cmd PlaybackCommand) error {
	user := auth.GetUser(c)
	if user == nil {
		return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
	}

	id := c.Param("id")
	session, err := h.store.GetSession(id)
	if err != nil {
		return echo.NewHTTPError(http.StatusNotFound, err.Error())
	}
	if session.UserID != user.UserID {
		return echo.NewHTTPError(http.StatusForbidden, "access denied")
	}

	session.State = state
	if err := h.store.UpdateSession(session); err != nil {
		return echo.NewHTTPError(http.StatusInternalServerError, "failed to update session: "+err.Error())
	}

	h.publish(session.ID, cmd, session.Position, session.Volume)
	return c.JSON(http.StatusOK, session)
}

func (h *Handler) publish(sessionID string, cmd PlaybackCommand, position int64, volume int) {
	if h.publisher == nil {
		return
	}
	h.publisher(PlaybackEvent{
		SessionID: sessionID,
		Command:   cmd,
		Position:  position,
		Volume:    volume,
		Timestamp: time.Now().UTC(),
	})
}
