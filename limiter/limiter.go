package limiter

import (
	"context"
	"net/http"
	"strings"
	"time"

	libredis "github.com/redis/go-redis/v9"
	limiterlib "github.com/ulule/limiter/v3"
	sredis "github.com/ulule/limiter/v3/drivers/store/redis"
)

const RateLimitErrorMessage = "You have reached maximum request limit."

var (
	// Global limiter store
	redisStore limiterlib.Store
	// Global func to fetch from request context
	FetchFromContext FuncFetchFromContext
	// Additional context per request
	AdditionalContext FuncFetchParamFromContext
)

type Limiter struct {
	// Maximum number of requests to limit per ttl
	expiry, globalExpiry ExpirableOptions

	// List of places to look up IP address
	// Default is "CF-Connecting-IP", "RemoteAddr", "X-Forwarded-For", "X-Real-IP"
	// You can rearrange the order as you like
	ipLookups []string

	// List of HTTP Methods to limit (GET, POST, PUT, etc.)
	// Empty means limit all methods
	methods map[string]bool

	// List of query params to consider, all
	// non-empty values will be considered
	queryParams []string

	// Consider sid if present in context
	includeUserId bool

	// HTTP message when limit is reached.
	message string

	userIdFromContext FuncFetchFromContext

	// Path specific additional context per request
	additionalContextFunc FuncFetchParamFromContext

	additionalContextParams []string

	// Ignore URL on the rate limiter keys
	ignoreURL bool

	limiter, globalLimiter *limiterlib.Limiter
	// Pluggable limiters allows support to override supported limits by `limiter`
	pluggableLimiter *PluggableLimiter
}

func Init(config LimiterOptions) {
	FetchFromContext = config.FuncFetchFromContext
	AdditionalContext = config.FuncAdditionalContext

	client := libredis.NewClient(config.Redis)

	storeOptions := limiterlib.StoreOptions{
		MaxRetry: 3,
		Prefix:   config.Prefix,
	}

	// Create a store with the redis client
	var err error
	redisStore, err = sredis.NewStoreWithOptions(client, storeOptions)
	if err != nil {
		panic("Err while init ratelimit redisStore: " + err.Error())
	}
}

func New(generalExpirableOptions *ExpirableOptions) *Limiter {
	lmt := &Limiter{}

	if generalExpirableOptions != nil {
		if generalExpirableOptions.DefaultExpirationTTL != 0 {
			lmt.SetTtl(generalExpirableOptions.DefaultExpirationTTL)
		}
		if generalExpirableOptions.ExpireJobInterval != 0 {
			lmt.SetLimits(generalExpirableOptions.ExpireJobInterval)
		}
	}
	if lmt.GetTtl() == 0 {
		lmt.SetTtl(1 * time.Minute)
	}
	if lmt.GetLimits() == 0 {
		lmt.SetLimits(5)
	}

	// Global limits will be valid for a ip across all the requests,
	// except for logged in users
	if lmt.GetGlobalTtl() == 0 {
		lmt.SetGlobalTtl(1 * time.Minute)
	}
	if lmt.GetGlobalLimits() == 0 {
		lmt.SetGlobalLimits(12)
	}

	lmt.SetIPLookups([]string{"CF-Connecting-IP", "X-Forwarded-For", "RemoteAddr", "X-Real-IP"})

	lmt.SetIncludeUserId(FetchFromContext != nil)

	lmt.SetUserIdFromContext(FetchFromContext)

	lmt.SetAdditionalContextFunc(AdditionalContext)

	lmt.SetErrorMessage(RateLimitErrorMessage)

	lmt.limiter = &limiterlib.Limiter{
		Store: redisStore,
		Rate: limiterlib.Rate{
			Period: lmt.GetTtl(),
			Limit:  lmt.GetLimits(),
		},
	}

	lmt.globalLimiter = &limiterlib.Limiter{
		Store: redisStore,
		Rate: limiterlib.Rate{
			Period: lmt.GetGlobalTtl(),
			Limit:  lmt.GetGlobalLimits(),
		},
	}

	return lmt
}

// SetIPLookups for setting limit of allowed requests for ttl
func (l *Limiter) SetLimits(limit int64) *Limiter {
	l.expiry.ExpireJobInterval = limit
	return l
}

func (l *Limiter) GetLimits() int64 {
	return l.expiry.ExpireJobInterval
}

// SetIPLookups for setting list of places to look up IP address
func (l *Limiter) SetTtl(ttl time.Duration) *Limiter {
	l.expiry.DefaultExpirationTTL = ttl
	return l
}

func (l *Limiter) GetTtl() time.Duration {
	return l.expiry.DefaultExpirationTTL
}

func (l *Limiter) SetGlobalLimits(limit int64) *Limiter {
	l.globalExpiry.ExpireJobInterval = limit
	return l
}

func (l *Limiter) GetGlobalLimits() int64 {
	return l.globalExpiry.ExpireJobInterval
}

// SetIPLookups for setting list of places to look up IP address
func (l *Limiter) SetGlobalTtl(ttl time.Duration) *Limiter {
	l.globalExpiry.DefaultExpirationTTL = ttl
	return l
}

func (l *Limiter) GetGlobalTtl() time.Duration {
	return l.globalExpiry.DefaultExpirationTTL
}

// SetIPLookups for setting list of places to look up IP address
func (l *Limiter) SetIPLookups(ipLookups []string) *Limiter {
	l.ipLookups = ipLookups
	return l
}

func (l *Limiter) GetIPLookups() []string {
	return l.ipLookups
}

// SetMethods for setting list of HTTP Methods to limit (GET, POST, PUT, etc.)
func (l *Limiter) SetMethods(methods []string) *Limiter {
	for _, method := range methods {
		l.methods[strings.ToUpper(method)] = true
	}
	return l
}

// GetMethod key according to list of HTTP Methods to limit (GET, POST, PUT, etc.)
func (l *Limiter) GetMethods(r *http.Request) string {
	if len(l.methods) == 0 {
		return r.Method
	}

	// if valid method exists, add to key
	if val, exists := l.methods[r.Method]; exists && val {
		return r.Method
	}
	return ""
}

// SetMethods for setting queryParams present in request
func (l *Limiter) SetQueryParams(queryParams []string) *Limiter {
	l.queryParams = queryParams
	return l
}

// set helper function to fetch student sid from context
func (l *Limiter) SetUserIdFromContext(f func(r *http.Request) (string, error)) *Limiter {
	l.userIdFromContext = f
	return l
}

// helper to fetch sid from context
func (l *Limiter) GetUserIdFromContext(r *http.Request) (string, error) {
	if l.userIdFromContext == nil {
		return "", nil
	}
	return l.userIdFromContext(r)
}

func (l *Limiter) SetAdditionalContextFunc(f FuncFetchParamFromContext) *Limiter {
	l.additionalContextFunc = f
	return l
}

func (l *Limiter) SetAdditionalContextParam(params ...string) *Limiter {
	l.additionalContextParams = params
	return l
}

func (l *Limiter) GetAdditionalContextParam(r *http.Request) (string, error) {
	return l.additionalContextFunc(r, l.additionalContextParams)
}

func (l *Limiter) SetPluggableLimiter(eo []ExpirableOptions) *Limiter {
	l.pluggableLimiter = NewPluggableLimiter(eo)
	return l
}

// SetMethods for setting includeUserId from context in request
func (l *Limiter) SetIncludeUserId(includeUserId bool) *Limiter {
	l.includeUserId = includeUserId
	return l
}

func (l *Limiter) GetIncludeUserId() bool {
	return l.includeUserId
}

func (l *Limiter) SetErrorMessage(message string) *Limiter {
	l.message = message
	return l
}

func (l *Limiter) GetErrorMessage() string {
	return l.message
}

// SetIgnoreURL for setting whenever to ignore the URL on rate limit keys
func (l *Limiter) SetIgnoreURL(enabled bool) *Limiter {
	l.ignoreURL = enabled
	return l
}

// GetIgnoreURL to determine if rate limit on url is required
func (l *Limiter) GetIgnoreURL() bool {
	return l.ignoreURL
}

// Validates if limiter has been successfully initialised to facilitate silent failover otherwise
func (l *Limiter) IsInitialised() bool {
	return l.limiter.Store != nil
}

func (l *Limiter) LimitReached(ctx context.Context, key string) (Context, error) {
	if !l.IsInitialised() {
		return Context{}, nil
	}

	lctx, err := l.limiter.Get(ctx, key)
	if err != nil {
		return Context{}, err
	}

	return Context(lctx), err
}

func (l *Limiter) GlobalLimitReached(ctx context.Context, key string) (Context, error) {
	if !l.IsInitialised() {
		return Context{}, nil
	}

	lctx, err := l.globalLimiter.Get(ctx, key)
	if err != nil {
		return Context{}, err
	}

	return Context(lctx), err
}
