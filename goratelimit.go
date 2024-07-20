package goratelimit

import (
	"net/http"
	"time"

	"github.com/alter123/go-ratelimit/libstring"
	"github.com/alter123/go-ratelimit/limiter"
)

func Init(config limiter.LimiterOptions) {
	limiter.Init(config)
}

// NewLimiter is a convenience function to limiter.New, returns limiter
// with default options, additional params can be added via method chaining
func NewLimiter(max int64, ttl time.Duration) *limiter.Limiter {
	goOptions := &limiter.ExpirableOptions{
		DefaultExpirationTTL: ttl,
		ExpireJobInterval:    max,
	}
	return limiter.New(goOptions)
}

func ExpirableOptions(eo ...limiter.ExpirableOptions) []limiter.ExpirableOptions {
	return eo
}

func NewExpirableOption(max int64, ttl time.Duration, suffix string) limiter.ExpirableOptions {
	return limiter.ExpirableOptions{
		DefaultExpirationTTL: ttl,
		ExpireJobInterval:    max,
		Suffix:               suffix,
	}
}

// BuildKeys generates a slice of keys to rate-limit by given limiter and request structs.
func BuildKeys(lmt *limiter.Limiter, r *http.Request) *limiter.LimiterKeys {
	remoteIP := libstring.RemoteIP(lmt.GetIPLookups(), 100, r)
	path := r.URL.Path
	limiterKeys := &limiter.LimiterKeys{}

	userIdToLimit := ""
	// Add to context if sid is set in request
	// Authenticated requests can override global limits
	if lmt.GetIncludeUserId() {
		if userId, err := lmt.GetUserIdFromContext(r); err == nil {
			userIdToLimit = userId
		}
	}

	// Global limits are valid only for non loggedin requests
	if len(userIdToLimit) == 0 {
		limiterKeys.Global = []string{remoteIP}
	}

	sliceKey := []string{remoteIP}
	if !lmt.GetIgnoreURL() {
		sliceKey = append(sliceKey, path)
	}

	lmtMethod := lmt.GetMethods(r)

	// Add additinal context if available
	additionalContext, _ := lmt.GetAdditionalContextParam(r)

	sliceKey = append(sliceKey, lmtMethod)
	sliceKey = append(sliceKey, userIdToLimit)
	sliceKey = append(sliceKey, additionalContext)
	limiterKeys.Request = sliceKey

	return limiterKeys
}

func ShouldSkipLimiter(lmt *limiter.Limiter, r *http.Request) bool {
	// Filter by remote ip
	// If we are unable to find remoteIP, skip limiter
	remoteIPPresent := libstring.RemoteIP(lmt.GetIPLookups(), 100, r) != ""

	// Filter if token is present in context
	userIdPresent := false
	if lmt.GetIncludeUserId() {
		if userId, err := lmt.GetUserIdFromContext(r); err == nil {
			userIdPresent = userId != ""
		}
	}

	return !remoteIPPresent && !userIdPresent
}

// LimitByRequest builds keys based on http.Request struct,
// loops through all the keys, and check if any one of them returns HTTPError.
func LimitByRequest(lmt *limiter.Limiter, r *http.Request) (limiter.Context, error) {
	var err error
	var lmtCtx limiter.Context

	shouldSkip := ShouldSkipLimiter(lmt, r)
	if shouldSkip {
		return lmtCtx, nil
	}

	sliceKeys := BuildKeys(lmt, r)

	if sliceKeys.IsGlobalValid() {
		lmtCtx, err = sliceKeys.Global.PluggableLimits(r.Context(), lmt)
		if err != nil || lmtCtx.LimitReached() {
			return lmtCtx, err
		}

		lmtCtx, err = sliceKeys.Global.GlobalLimits(r.Context(), lmt)
		if err != nil || lmtCtx.LimitReached() {
			return lmtCtx, err
		}
	}

	return sliceKeys.Request.Limits(r.Context(), lmt)
}
