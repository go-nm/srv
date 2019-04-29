package srv_test

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"syscall"
	"testing"
	"time"

	"github.com/julienschmidt/httprouter"
	"github.com/stretchr/testify/assert"

	"github.com/go-nm/srv"
)

const defaultAddr = ":9876"

func TestNew(t *testing.T) {
	t.Run("Success", testNew_Success)
	t.Run("RouteHandler", testNew_RouteHandler)
	t.Run("ContextPath", testNew_ContextPath)
}

func testNew_Success(t *testing.T) {
	// Arrange
	assert := assert.New(t)

	// Act
	got := srv.New()

	// Assert
	routesHandler, _, _ := got.Lookup("GET", "/_system/routes")
	readinessHandler, _, _ := got.Lookup("GET", "/_system/readiness")
	livenessHandler, _, _ := got.Lookup("GET", "/_system/liveness")
	infoHandler, _, _ := got.Lookup("GET", "/_system/info")

	assert.NotNil(got.Router)
	assert.NotNil(got.Negroni)
	assert.Nil(routesHandler)
	assert.NotNil(readinessHandler)
	assert.NotNil(livenessHandler)
	assert.NotNil(infoHandler)
}

func testNew_RouteHandler(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	type args struct {
		env string
	}
	tests := []struct {
		name               string
		args               args
		wantRoutesEndpoint bool
	}{
		{name: "Dev", args: args{env: "dev"}, wantRoutesEndpoint: true},
		{name: "Test", args: args{env: "test"}, wantRoutesEndpoint: true},
		{name: "UAT", args: args{env: "uat"}},
		{name: "IT", args: args{env: "it"}},
		{name: "Prod", args: args{env: "prod"}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Act
			got := srv.New(srv.OptionAppEnv(tt.args.env))

			// Assert
			routesHandler, _, _ := got.Lookup("GET", "/_system/routes")
			assert.Equal(tt.wantRoutesEndpoint, (routesHandler != nil))
		})
	}
}

func testNew_ContextPath(t *testing.T) {
	// Arrange
	assert := assert.New(t)

	// Act
	got := srv.New(srv.OptionContextPath("/testing/path/"))

	// Assert
	infoHandler, _, _ := got.Lookup("GET", "/testing/path/_system/info")
	assert.NotNil(infoHandler)
}

func Testsrv_AddLivenessCheck(t *testing.T) {
	// Arrange
	checkName := "testCheck"
	assert := assert.New(t)
	srv := srv.New()

	// Act
	srv.AddLivenessCheck(checkName, func() srv.HealthMetricResult {
		return srv.HealthMetricResult{OK: true}
	})

	// Assert
	var parsedRes srv.HealthResponse
	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/_system/liveness", nil)
	srv.Router.ServeHTTP(res, req)
	json.NewDecoder(res.Result().Body).Decode(&parsedRes)
	assert.Equal("ok", parsedRes.Metrics[checkName].Status)
}

func Testsrv_AddReadinessCheck(t *testing.T) {
	// Arrange
	checkName := "testCheck"
	assert := assert.New(t)
	srv := srv.New()

	// Act
	srv.AddReadinessCheck(checkName, func() srv.HealthMetricResult {
		return srv.HealthMetricResult{OK: true}
	})

	// Assert
	var parsedRes srv.HealthResponse
	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/_system/readiness", nil)
	srv.Router.ServeHTTP(res, req)
	json.NewDecoder(res.Result().Body).Decode(&parsedRes)
	assert.Equal("ok", parsedRes.Metrics[checkName].Status)
}

func Testsrv_Run(t *testing.T) {
	t.Run("Success", testsrv_Run_Success)
	t.Run("srvStopSignalSuccess", testsrv_Run_StopSignalSuccess)
	t.Run("srvRunningError", testsrv_Run_srvRunningError)
	t.Run("srvListenError", testsrv_Run_ListenError)
}

func testsrv_Run_Success(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	var err error
	s := srv.New()

	// Act
	go func() { err = s.Run(defaultAddr) }()
	defer s.Shutdown()
	waitForAddr(defaultAddr)

	// Assert
	assert.Nil(err)
}

func testsrv_Run_StopSignalSuccess(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	var err error
	doneCh := make(chan bool)
	s := srv.New()

	go func() {
		err = s.Run(defaultAddr)
		doneCh <- true
	}()
	defer s.Shutdown()
	waitForAddr(defaultAddr)
	assert.Nil(err)

	// Act
	syscall.Kill(syscall.Getpid(), syscall.SIGTERM)
	<-doneCh

	// Assert
	assert.Nil(err)
}

func testsrv_Run_srvRunningError(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	var err error
	s := srv.New()

	go func() { err = s.Run(defaultAddr) }()
	defer s.Shutdown()
	waitForAddr(defaultAddr)
	assert.Nil(err)

	// Act
	go func() { err = s.Run(defaultAddr) }()
	waitForAddr(defaultAddr)

	// Assert
	assert.Equal(err, srv.ErrsrvAlreadyRunning)
}

func testsrv_Run_ListenError(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	var err error

	s1 := srv.New()
	go func() { err = s1.Run(defaultAddr) }()
	defer s1.Shutdown()
	waitForAddr(defaultAddr)
	assert.Nil(err)

	// Act
	s2 := srv.New()
	go func() { err = s2.Run(defaultAddr) }()
	defer s2.Shutdown()
	waitForAddr(defaultAddr)

	// Assert
	assert.NotNil(err)
	assert.Equal(err.Error(), "common/srv: failed to start srv: listen tcp "+defaultAddr+": bind: address already in use")
}

func Testsrv_IsRunning(t *testing.T) {
	t.Run("Running", testsrv_IsRunning_Running)
	t.Run("Stopped", testsrv_IsRunning_Stopped)
}

func testsrv_IsRunning_Running(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	var err error
	s := srv.New()

	go func() { err = s.Run(defaultAddr) }()
	defer s.Shutdown()
	waitForAddr(defaultAddr)
	assert.Nil(err)

	// Act
	status := s.IsRunning()

	// Assert
	assert.True(status)
}

func testsrv_IsRunning_Stopped(t *testing.T) {
	// Arrange
	assert := assert.New(t)

	// Act
	status := srv.New().IsRunning()

	// Assert
	assert.False(status)
}

func Testsrv_Shutdown(t *testing.T) {
	t.Run("Success", testsrv_Shutdown_Success)
	t.Run("NotRunning", testsrv_Shutdown_NotRunning)
}

func testsrv_Shutdown_Success(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	var err error

	s := srv.New()
	go func() { err = s.Run(defaultAddr) }()
	waitForAddr(defaultAddr)
	assert.Nil(err)

	// Act
	err = s.Shutdown()

	// Assert
	assert.Nil(err)
}

func testsrv_Shutdown_NotRunning(t *testing.T) {
	// Arrange
	assert := assert.New(t)

	// Act
	err := srv.New().Shutdown()

	// Assert
	assert.Equal(err, srv.ErrsrvStopped)
}

func Testsrv_Handle(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	srv := srv.New()

	// Act
	srv.Handle("GET", "/testpath", func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {})

	// Assert
	handler, _, _ := srv.Lookup("GET", "/testpath")
	assert.NotNil(handler)
}

func Testsrv_GET(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	srv := srv.New()

	// Act
	srv.GET("/testpath", func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {})

	// Assert
	handler, _, _ := srv.Lookup("GET", "/testpath")
	assert.NotNil(handler)
}

func Testsrv_POST(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	srv := srv.New()

	// Act
	srv.POST("/testpath", func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {})

	// Assert
	handler, _, _ := srv.Lookup("POST", "/testpath")
	assert.NotNil(handler)
}

func Testsrv_PUT(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	srv := srv.New()

	// Act
	srv.PUT("/testpath", func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {})

	// Assert
	handler, _, _ := srv.Lookup("PUT", "/testpath")
	assert.NotNil(handler)
}

func Testsrv_PATCH(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	srv := srv.New()

	// Act
	srv.PATCH("/testpath", func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {})

	// Assert
	handler, _, _ := srv.Lookup("PATCH", "/testpath")
	assert.NotNil(handler)
}

func Testsrv_DELETE(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	srv := srv.New()

	// Act
	srv.DELETE("/testpath", func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {})

	// Assert
	handler, _, _ := srv.Lookup("DELETE", "/testpath")
	assert.NotNil(handler)
}

func Testsrv_HEAD(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	srv := srv.New()

	// Act
	srv.HEAD("/testpath", func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {})

	// Assert
	handler, _, _ := srv.Lookup("HEAD", "/testpath")
	assert.NotNil(handler)
}

func Testsrv_OPTIONS(t *testing.T) {
	// Arrange
	assert := assert.New(t)
	srv := srv.New()

	// Act
	srv.OPTIONS("/testpath", func(w http.ResponseWriter, r *http.Request, ps httprouter.Params) {})

	// Assert
	handler, _, _ := srv.Lookup("OPTIONS", "/testpath")
	assert.NotNil(handler)
}

func waitForAddr(addr string) {
	port := strings.Split(addr, ":")[1]
	for i := 1; i <= 10; i++ {
		conn, _ := net.DialTimeout("tcp", net.JoinHostPort("", port), 100*time.Millisecond)
		if conn != nil {
			conn.Close()
			break
		}
	}
}
