package serverfiber

import "github.com/gofiber/fiber/v2"

type Router interface {
	Register(app *fiber.App)
}
