package router

import (
	"net/http"
	"time"

	"github.com/Allmight-456/ticketflow/internal/handler"
	"github.com/Allmight-456/ticketflow/internal/middleware"
	"github.com/Allmight-456/ticketflow/internal/repository/cache"
	"github.com/Allmight-456/ticketflow/internal/service"
	"github.com/go-chi/chi/v5"
	chiMiddleware "github.com/go-chi/chi/v5/middleware"
)

// New wires all handlers and middleware into a Chi router.
//
// Middleware chain (outermost → innermost):
//   - zerolog request logger
//   - panic recoverer
//   - global rate limiter (100 req/min per IP)
//   - per-route JWT auth + RBAC gates
func New(
	authSvc *service.AuthService,
	eventSvc *service.EventService,
	bookingSvc *service.BookingService,
	auditSvc *service.AuditService,
	redisRepo *cache.RedisRepo,
) http.Handler {
	r := chi.NewRouter()

	// ── Global middleware ──────────────────────────────────────────────────
	r.Use(chiMiddleware.RequestID)
	r.Use(chiMiddleware.RealIP)
	r.Use(chiMiddleware.Recoverer)
	r.Use(chiMiddleware.Timeout(30 * time.Second))
	r.Use(middleware.RateLimiter(redisRepo, time.Minute, 100))

	// ── Handlers ───────────────────────────────────────────────────────────
	authH := handler.NewAuthHandler(authSvc)
	eventH := handler.NewEventHandler(eventSvc)
	bookingH := handler.NewBookingHandler(bookingSvc)
	auditH := handler.NewAuditHandler(auditSvc)

	auth := middleware.Authenticate(authSvc)
	adminOnly := middleware.RequireAdmin

	// ── Public routes ──────────────────────────────────────────────────────
	r.Post("/auth/register", authH.Register)
	r.Post("/auth/login", authH.Login)

	// ── Health check ───────────────────────────────────────────────────────
	r.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte(`{"status":"ok"}`))
	})

	// ── Authenticated routes ───────────────────────────────────────────────
	r.Group(func(r chi.Router) {
		r.Use(auth)

		// Events — read is open to all authenticated users
		r.Get("/events", eventH.List)
		r.Get("/events/{id}", eventH.GetByID)

		// Events — write is admin-only
		r.Group(func(r chi.Router) {
			r.Use(adminOnly)
			r.Post("/events", eventH.Create)
			r.Put("/events/{id}", eventH.Update) // ID in URL path (preferred)
			r.Put("/events", eventH.Update)      // ID in request body (fallback)
			r.Delete("/events/{id}", eventH.Delete)
			r.Post("/events/batch", eventH.BatchCreate)
		})

		// Bookings — authenticated users can book and cancel their own
		r.Post("/bookings", bookingH.Create)
		r.Delete("/bookings/{id}", bookingH.Cancel)
		r.Post("/bookings/batch", bookingH.BatchCreate)

		// Audit trail — admin-only
		r.With(adminOnly).Get("/audit/{resource_type}/{resource_id}", auditH.GetHistory)
	})

	return r
}
