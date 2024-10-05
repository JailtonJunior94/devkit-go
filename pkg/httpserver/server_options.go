package httpserver

const (
	defaultHTTPPort = "80"
)

var (
	defaultSettings = settings{
		port:         defaultHTTPPort,
		errorHandler: defaultHandleError,
	}
)

type (
	Option   func(s settings) settings
	settings struct {
		port              string
		routes            []Route
		globalMiddlewares []Middleware
		errorHandler      ErrorHandler
	}
)

func WithPort(port string) Option {
	return func(s settings) settings {
		s.port = port
		return s
	}
}

func WithRoutes(routes ...Route) Option {
	return func(s settings) settings {
		s.routes = append(s.routes, routes...)
		return s
	}
}

func WithMiddlewares(middlewares ...Middleware) Option {
	return func(s settings) settings {
		s.globalMiddlewares = append(s.globalMiddlewares, middlewares...)
		return s
	}
}

func WithErrorHandler(handler ErrorHandler) Option {
	return func(s settings) settings {
		s.errorHandler = handler
		return s
	}
}
