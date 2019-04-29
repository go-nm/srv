package srv

type optionName int

const (
	optionContextPath optionName = iota
	optionAppEnv
)

// Option is the struct for server based options
type Option struct {
	name  optionName
	value interface{}
}

// OptionContextPath is used to set a prefix to the path of all requests on the server.
// This value must be passed in when creating a new instance of the Server.
// If using the context path you can always set a route to outside of the context path
// by directly calling one of the Router methods on the Server struct.
func OptionContextPath(path string) Option {
	return Option{name: optionContextPath, value: path}
}

// OptionAppEnv is used to set specific security runtime environment variables.
// When value is dev the panic handler is disabled to view stacktraces in the browser
// and the /_system/routes route is avaliable to show all routes that were registered
// with the server.
func OptionAppEnv(envName string) Option {
	return Option{name: optionAppEnv, value: envName}
}
