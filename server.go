package srv

import (
	"context"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/urfave/negroni"
)

// gracefulTermTimeout is the amount of time to wait for all HTTP requests
// to complete before forcing the server to shut down
const gracefulTermTimeout = 30 * time.Second

// stopSignals contains all of the OS Signals to respond to during
// a graceful shutdown of the HTTP server.
var stopSignals = []os.Signal{
	syscall.SIGHUP,
	syscall.SIGINT,
	syscall.SIGQUIT,
	syscall.SIGTERM,
}

// ErrServerStopped is the error returned when the server is not running
var ErrServerStopped = errors.New("common/server: not running")

// ErrServerAlreadyRunning is the error returned when the server is already running
var ErrServerAlreadyRunning = errors.New("common/server: already running")

// Server struct containing the httprouter routing handler
// and the negroni middleware framework
type Server struct {
	*httprouter.Router
	*negroni.Negroni

	contextPath string

	routes []RouteInfo

	httpServer       *http.Server
	readinessMetrics []HealthMetric
	livenessMetrics  []HealthMetric
	infoMetrics      []InfoMetric
}

// New creates a new instance of the router. Context path is the prefix to all url paths.
func New(opts ...Option) *Server {
	srv := &Server{Router: httprouter.New(), Negroni: negroni.Classic()}

	srv.HandleMethodNotAllowed = true
	srv.MethodNotAllowed = MethodNotAllowedHandler()
	srv.NotFound = NotFoundHandler()
	srv.PanicHandler = PanicHandler()

	for _, o := range opts {
		switch o.name {
		case optionContextPath:
			srv.contextPath = strings.TrimSuffix(o.value.(string), "/")
		case optionAppEnv:
			if o.value == "dev" || o.value == "test" {
				srv.GET("/_system/routes", RouteHandler(&srv.routes))
				srv.PanicHandler = nil
			}
		}
	}

	srv.GET("/_system/readiness", HealthHandler(&srv.readinessMetrics))
	srv.GET("/_system/liveness", HealthHandler(&srv.livenessMetrics))
	srv.GET("/_system/info", InfoHandler(&srv.infoMetrics))

	return srv
}

// AddLivenessCheck to the list of liveness metrics used to validate the system
// is running and healthy at the /_system/liveness endpoint. The name parameter
// should be camel-case. Liveness metrics are defined as the server has moved into
// a broken state and cannot recover except by being restarted.
func (s *Server) AddLivenessCheck(name string, handler HealthMetricHandler) {
	s.livenessMetrics = append(s.livenessMetrics, HealthMetric{Name: name, GetValue: handler})
}

// AddReadinessCheck to the list of readiness metrics used to validate the system
// is running and healthy at the /_system/readiness endpoint. The name parameter
// should be camel-case. Readiness metrics are used when the server is temporarily unable
// to serve traffic this is different from liveness in that readiness should not restart
// the application when it is failing.
func (s *Server) AddReadinessCheck(name string, handler HealthMetricHandler) {
	s.readinessMetrics = append(s.readinessMetrics, HealthMetric{Name: name, GetValue: handler})
}

// AddInfoMetric to the list of info metrics used to get info about the running system
// at the /_system/info endpoint. The name parameter should be camel-case.
func (s *Server) AddInfoMetric(name string, handler InfoMetricHandler) {
	s.infoMetrics = append(s.infoMetrics, InfoMetric{Name: name, GetValue: handler})
}

// Run the HTTP server on the addr provided with graceful shutdown
func (s *Server) Run(addr string) error {
	// If the server is already running return error
	if s.httpServer != nil {
		return ErrServerAlreadyRunning
	}

	s.Negroni.UseHandler(s.Router)

	s.httpServer = &http.Server{Addr: addr, Handler: s.Negroni}

	// Start the server in a gorutine
	errChan := make(chan error)
	go s.startServer(errChan)

	// Wait for an OS Signal
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, stopSignals...)

	// Wait for either the start error or the OS signal
	select {
	case <-stop:
		return s.Shutdown()

	case err := <-errChan:
		return err
	}
}

// Shutdown gracefully stops the HTTP server
func (s *Server) Shutdown() (err error) {
	if s.httpServer == nil {
		return ErrServerStopped
	}

	log.Println("Shutting down HTTP server...")

	// Create a timeout context to force kill requests if they take more than an allotted time
	ctx, cancel := context.WithTimeout(context.Background(), gracefulTermTimeout)
	defer cancel()

	err = s.httpServer.Shutdown(ctx)
	s.httpServer = nil
	return err
}

// IsRunning tells if the server is currently running
func (s Server) IsRunning() bool {
	return s.httpServer != nil
}

// start the server and fatally log failure if there is a failure starting the server
func (s *Server) startServer(errChan chan error) {
	// Start the server
	log.Printf("Starting HTTP server at %s\n", s.httpServer.Addr)
	err := s.httpServer.ListenAndServe()

	// Log error if the server was not closed
	if err != nil && err != http.ErrServerClosed {
		s.httpServer = nil
		errChan <- errors.New("common/server: failed to start server: " + err.Error())
	}
}

// Handle is a function that can be registered to a route to handle HTTP requests.
// Like http.HandlerFunc, but has a third parameter for the values of wildcards (variables).
func (s *Server) Handle(method, path string, handle httprouter.Handle) {
	s.routes = append(s.routes, RouteInfo{Method: method, Path: s.contextPath + path})
	s.Router.Handle(method, s.contextPath+path, handle)
}

// GET is a shortcut for router.Handle("GET", path, handle)
func (s *Server) GET(path string, handle httprouter.Handle) {
	s.Handle("GET", path, handle)
}

// POST is a shortcut for router.Handle("POST", path, handle)
func (s *Server) POST(path string, handle httprouter.Handle) {
	s.Handle("POST", path, handle)
}

// PUT is a shortcut for router.Handle("PUT", path, handle)
func (s *Server) PUT(path string, handle httprouter.Handle) {
	s.Handle("PUT", path, handle)
}

// PATCH is a shortcut for router.Handle("PATCH", path, handle)
func (s *Server) PATCH(path string, handle httprouter.Handle) {
	s.Handle("PATCH", path, handle)
}

// DELETE is a shortcut for router.Handle("DELETE", path, handle)
func (s *Server) DELETE(path string, handle httprouter.Handle) {
	s.Handle("DELETE", path, handle)
}

// HEAD is a shortcut for router.Handle("HEAD", path, handle)
func (s *Server) HEAD(path string, handle httprouter.Handle) {
	s.Handle("HEAD", path, handle)
}

// OPTIONS is a shortcut for router.Handle("OPTIONS", path, handle)
func (s *Server) OPTIONS(path string, handle httprouter.Handle) {
	s.Handle("OPTIONS", path, handle)
}
