package http

import (
	"crypto/tls"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/stretchr/testify/suite"
)

type MiddlewareSuite struct {
	suite.Suite
}

func (s *MiddlewareSuite) TestGetClientIP() {
	s.Run("extracts IP from X-Forwarded-For header", func() {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.1, 198.51.100.1")
		req.RemoteAddr = "192.0.2.1:12345"

		ip := getClientIP(req)
		s.Equal("203.0.113.1", ip)
	})

	s.Run("extracts IP from X-Real-IP header", func() {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Real-IP", "203.0.113.1")
		req.RemoteAddr = "192.0.2.1:12345"

		ip := getClientIP(req)
		s.Equal("203.0.113.1", ip)
	})

	s.Run("falls back to RemoteAddr", func() {
		req := httptest.NewRequest("GET", "/test", nil)
		req.RemoteAddr = "192.0.2.1:12345"

		ip := getClientIP(req)
		s.Equal("192.0.2.1", ip)
	})

	s.Run("handles single IP in X-Forwarded-For", func() {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-For", "203.0.113.1")
		req.RemoteAddr = "192.0.2.1:12345"

		ip := getClientIP(req)
		s.Equal("203.0.113.1", ip)
	})
}

func (s *MiddlewareSuite) TestGetHTTPRoute() {
	s.Run("returns exact match for known routes", func() {
		routes := []string{"/healthz", "/mcp", "/sse", "/message", "/stats", "/metrics"}
		for _, route := range routes {
			result := getHTTPRoute(route)
			s.Equal(route, result)
		}
	})

	s.Run("normalizes well-known paths", func() {
		paths := []string{
			"/.well-known/oauth-authorization-server",
			"/.well-known/openid-configuration",
			"/.well-known/other",
		}
		for _, path := range paths {
			result := getHTTPRoute(path)
			s.Equal("/.well-known/*", result)
		}
	})

	s.Run("returns path as-is for unknown routes", func() {
		path := "/some/unknown/path"
		result := getHTTPRoute(path)
		s.Equal(path, result)
	})
}

func (s *MiddlewareSuite) TestGetScheme() {
	s.Run("returns https for TLS connection", func() {
		req := httptest.NewRequest("GET", "/test", nil)
		// Create a TLS connection state to indicate HTTPS
		req.TLS = &tls.ConnectionState{}

		scheme := getScheme(req)
		s.Equal("https", scheme)
	})

	s.Run("returns scheme from X-Forwarded-Proto header", func() {
		req := httptest.NewRequest("GET", "/test", nil)
		req.Header.Set("X-Forwarded-Proto", "https")

		scheme := getScheme(req)
		s.Equal("https", scheme)
	})

	s.Run("returns http by default", func() {
		req := httptest.NewRequest("GET", "/test", nil)

		scheme := getScheme(req)
		s.Equal("http", scheme)
	})
}

func (s *MiddlewareSuite) TestRequestMiddleware() {
	s.Run("allows request to pass through", func() {
		called := false
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})

		middleware := RequestMiddleware(handler)

		req := httptest.NewRequest("GET", "/mcp", nil)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		s.True(called)
		s.Equal(http.StatusOK, rec.Code)
	})

	s.Run("skips health check requests", func() {
		called := false
		handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			called = true
			w.WriteHeader(http.StatusOK)
		})

		middleware := RequestMiddleware(handler)

		req := httptest.NewRequest("GET", "/healthz", nil)
		rec := httptest.NewRecorder()

		middleware.ServeHTTP(rec, req)

		s.True(called)
		s.Equal(http.StatusOK, rec.Code)
	})
}

func (s *MiddlewareSuite) TestLoggingResponseWriter() {
	s.Run("captures status code from WriteHeader", func() {
		rec := httptest.NewRecorder()
		lrw := &loggingResponseWriter{
			ResponseWriter: rec,
			statusCode:     http.StatusOK,
		}

		lrw.WriteHeader(http.StatusCreated)

		s.Equal(http.StatusCreated, lrw.statusCode)
		s.True(lrw.headerWritten)
	})

	s.Run("captures status code from Write", func() {
		rec := httptest.NewRecorder()
		lrw := &loggingResponseWriter{
			ResponseWriter: rec,
			statusCode:     http.StatusOK,
		}

		lrw.Write([]byte("test"))

		s.Equal(http.StatusOK, lrw.statusCode)
		s.True(lrw.headerWritten)
	})

	s.Run("ignores duplicate WriteHeader calls", func() {
		rec := httptest.NewRecorder()
		lrw := &loggingResponseWriter{
			ResponseWriter: rec,
			statusCode:     http.StatusOK,
		}

		lrw.WriteHeader(http.StatusCreated)
		lrw.WriteHeader(http.StatusBadRequest)

		s.Equal(http.StatusCreated, lrw.statusCode)
	})

	s.Run("supports Flusher interface", func() {
		rec := httptest.NewRecorder()
		lrw := &loggingResponseWriter{
			ResponseWriter: rec,
			statusCode:     http.StatusOK,
		}

		// Should not panic
		lrw.Flush()
	})
}

func TestMiddlewareSuite(t *testing.T) {
	suite.Run(t, new(MiddlewareSuite))
}
