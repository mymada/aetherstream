package metrics

import (
	"net/http"
	"net/http/pprof"

	"github.com/labstack/echo/v4"
)

// RegisterPProf registers pprof endpoints on the given Echo instance under /debug/pprof
func RegisterPProf(e *echo.Echo) {
	// Index
	e.GET("/debug/pprof", echo.WrapHandler(http.HandlerFunc(pprof.Index)))
	// Profiles
	e.GET("/debug/pprof/:profile", func(c echo.Context) error {
		profile := c.Param("profile")
		switch profile {
		case "cmdline":
			return echo.WrapHandler(http.HandlerFunc(pprof.Cmdline))(c)
		case "profile":
			return echo.WrapHandler(http.HandlerFunc(pprof.Profile))(c)
		case "symbol":
			return echo.WrapHandler(http.HandlerFunc(pprof.Symbol))(c)
		case "trace":
			return echo.WrapHandler(http.HandlerFunc(pprof.Trace))(c)
		default:
			// goroutine, heap, threadcreate, block, mutex, etc.
			return echo.WrapHandler(pprof.Handler(profile))(c)
		}
	})
	e.GET("/debug/pprof/:profile/*", func(c echo.Context) error {
		profile := c.Param("profile")
		return echo.WrapHandler(pprof.Handler(profile))(c)
	})
}
