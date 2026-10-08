package stripeprototype

import (
	"encoding/json"
	"errors"
	"io"
	"mime"
	"net"
	"net/http"
	"net/url"
	"strings"

	"github.com/getkin/kin-openapi/openapi3"
	"github.com/go-chi/chi/v5"
	strictmiddleware "github.com/oapi-codegen/nethttp-middleware"
)

const maximumRequestBytes = 1 << 20

func NewHTTPHandler(service *Service, specification []byte) (http.Handler, error) {
	loader := openapi3.NewLoader()
	contract, err := loader.LoadFromData(specification)
	if err != nil {
		return nil, err
	}
	contract.Servers = nil
	router := chi.NewRouter()
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			host := r.Host
			if value, _, splitErr := net.SplitHostPort(host); splitErr == nil {
				host = value
			}
			if host != loopbackAddress && host != "localhost" {
				writeError(w, http.StatusForbidden, errors.New("local prototype requests only"))
				return
			}
			if strings.HasPrefix(r.URL.Path, "/api/") {
				origin := r.Header.Get("Origin")
				callback, _ := url.Parse(service.config.ReturnURL)
				if (origin != "" && origin != callback.Scheme+"://"+callback.Host) ||
					r.Header.Get("Sec-Fetch-Site") == "cross-site" {
					writeError(w, http.StatusForbidden, errors.New("cross-origin billing requests are refused"))
					return
				}
			}
			w.Header().Set("Cache-Control", "no-store")
			w.Header().Set("X-Content-Type-Options", "nosniff")
			r.Body = http.MaxBytesReader(w, r.Body, maximumRequestBytes)
			next.ServeHTTP(w, r)
		})
	})
	router.Get("/health", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, map[string]string{"status": "ok", "mode": "sandbox"})
	})
	router.Post("/webhooks/stripe", func(w http.ResponseWriter, r *http.Request) {
		body, readErr := io.ReadAll(r.Body)
		if readErr == nil {
			readErr = service.HandleWebhook(r.Context(), body, r.Header.Get("Stripe-Signature"))
		}
		if readErr != nil {
			writeError(w, http.StatusBadRequest, readErr)
			return
		}
		writeJSON(w, WebhookResponse{Received: true})
	})
	router.Group(func(api chi.Router) {
		api.Use(strictmiddleware.OapiRequestValidatorWithOptions(contract, &strictmiddleware.Options{
			ErrorHandler: func(w http.ResponseWriter, _ string, statusCode int) { writeError(w, statusCode, ErrInput) },
		}))
		api.Get("/api/state", func(w http.ResponseWriter, r *http.Request) {
			state, stateErr := service.State(r.Context())
			respond(w, state, stateErr)
		})
		api.Post("/api/prices", func(w http.ResponseWriter, r *http.Request) {
			var input CreatePriceRequest
			if !decodeJSON(w, r, &input) {
				return
			}
			price, createErr := service.CreatePrice(r.Context(), input)
			if createErr != nil {
				respond(w, nil, createErr)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			w.WriteHeader(http.StatusCreated)
			writeJSON(w, price)
		})
		api.Post("/api/prices/{priceId}/archive", func(w http.ResponseWriter, r *http.Request) {
			price, archiveErr := service.ArchivePrice(r.Context(), chi.URLParam(r, "priceId"))
			respond(w, price, archiveErr)
		})
		api.Put("/api/settings", func(w http.ResponseWriter, r *http.Request) {
			var input UpdateSettingsRequest
			if !decodeJSON(w, r, &input) {
				return
			}
			settings, settingsErr := service.UpdateSettings(r.Context(), input)
			respond(w, settings, settingsErr)
		})
		api.Post("/api/organizations/{organizationId}/checkout", func(w http.ResponseWriter, r *http.Request) {
			var input CheckoutRequest
			if !decodeJSON(w, r, &input) {
				return
			}
			result, checkoutErr := service.Checkout(r.Context(), chi.URLParam(r, "organizationId"), input)
			respond(w, result, checkoutErr)
		})
		api.Post("/api/organizations/{organizationId}/subscription", func(w http.ResponseWriter, r *http.Request) {
			var input SubscriptionRequest
			if !decodeJSON(w, r, &input) {
				return
			}
			result, updateErr := service.ChangeSubscription(r.Context(), chi.URLParam(r, "organizationId"), input)
			respond(w, result, updateErr)
		})
		api.Post("/api/organizations/{organizationId}/cancel", func(w http.ResponseWriter, r *http.Request) {
			result, cancelErr := service.SetCancellation(r.Context(), chi.URLParam(r, "organizationId"), true)
			respond(w, result, cancelErr)
		})
		api.Post("/api/organizations/{organizationId}/resume", func(w http.ResponseWriter, r *http.Request) {
			result, resumeErr := service.SetCancellation(r.Context(), chi.URLParam(r, "organizationId"), false)
			respond(w, result, resumeErr)
		})
		api.Post("/api/organizations/{organizationId}/sync", func(w http.ResponseWriter, r *http.Request) {
			result, syncErr := service.SyncOrganization(r.Context(), chi.URLParam(r, "organizationId"))
			respond(w, result, syncErr)
		})
		api.Post("/api/organizations/{organizationId}/portal", func(w http.ResponseWriter, r *http.Request) {
			result, portalErr := service.Portal(r.Context(), chi.URLParam(r, "organizationId"))
			respond(w, result, portalErr)
		})
	})
	return router, nil
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any) bool {
	contentType, _, _ := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if contentType != "application/json" {
		writeError(w, http.StatusBadRequest, ErrInput)
		return false
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		writeError(w, http.StatusBadRequest, ErrInput)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		writeError(w, http.StatusBadRequest, ErrInput)
		return false
	}
	return true
}

func respond(w http.ResponseWriter, value any, err error) {
	if err != nil {
		status := http.StatusBadGateway
		if errors.Is(err, ErrInput) {
			status = http.StatusBadRequest
		}
		if errors.Is(err, ErrConflict) {
			status = http.StatusConflict
		}
		writeError(w, status, err)
		return
	}
	writeJSON(w, value)
}

func writeJSON(w http.ResponseWriter, value any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(ErrorResponse{Error: err.Error()})
}
