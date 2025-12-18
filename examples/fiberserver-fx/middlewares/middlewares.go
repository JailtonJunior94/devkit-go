package middlewares

import (
	"github.com/JailtonJunior94/devkit-go/pkg/fiberserver"
	"go.uber.org/fx"
)

// Module provides all global middlewares.
var Module = fx.Module("middlewares",
	fx.Provide(fx.Annotate(
		provideRequestIDMiddleware,
		fx.ResultTags(`group:"middlewares"`),
	)),
	fx.Provide(fx.Annotate(
		provideRecoveryMiddleware,
		fx.ResultTags(`group:"middlewares"`),
	)),
	fx.Provide(fx.Annotate(
		provideSecurityHeadersMiddleware,
		fx.ResultTags(`group:"middlewares"`),
	)),
	fx.Provide(fx.Annotate(
		provideLoggerMiddleware,
		fx.ResultTags(`group:"middlewares"`),
	)),
	fx.Provide(fx.Annotate(
		provideJSONContentTypeMiddleware,
		fx.ResultTags(`group:"middlewares"`),
	)),
)

func provideRequestIDMiddleware() fiberserver.Middleware {
	return fiberserver.RequestID
}

func provideRecoveryMiddleware() fiberserver.Middleware {
	return fiberserver.Recovery
}

func provideSecurityHeadersMiddleware() fiberserver.Middleware {
	return fiberserver.SecurityHeaders
}

func provideLoggerMiddleware() fiberserver.Middleware {
	return fiberserver.Logger
}

func provideJSONContentTypeMiddleware() fiberserver.Middleware {
	return fiberserver.JSONContentType
}
