package limiter

import (
	"context"
	"net/http"
	"strings"
	"time"

	libredis "github.com/redis/go-redis/v9"
)

const (
	// Idenfier used while joining the keys for the store
	KeyJoinIdentifier = "|"
)

// Helper to fetch value from request context
// Client can provide a custom implementation to fetch value
// from request context or determine how to use context
type FuncFetchFromContext func(r *http.Request) (string, error)
type FuncFetchParamFromContext func(r *http.Request, params []string) (string, error)

// Options for limiter init
type LimiterOptions struct {
	FuncFetchFromContext  FuncFetchFromContext
	FuncAdditionalContext FuncFetchParamFromContext
	Redis                 *libredis.Options
	Prefix                string
}

// ExpirableOptions are options used for new limiter creation
type ExpirableOptions struct {
	DefaultExpirationTTL time.Duration
	ExpireJobInterval    int64
	Suffix               string
}

// Context is the limit context.
type Context struct {
	Limit     int64
	Remaining int64
	Reset     int64
	Reached   bool
}

func (c *Context) LimitReached() bool {
	return c.Reached
}

type LimiterKeysValue []string

type LimiterKeys struct {
	Global, Request LimiterKeysValue
}

func (l *LimiterKeys) IsGlobalValid() bool {
	return len(l.Global) > 0
}

func LimitByKeys(ctx context.Context, lmt *Limiter, keys []string) (Context, error) {
	return lmt.LimitReached(ctx, strings.Join(keys, KeyJoinIdentifier))
}

// LimitByKeys keeps track number of request made by keys separated by pipe.
// It returns request context which contains remaining requests count.
func (lv LimiterKeysValue) GlobalLimits(ctx context.Context, lmt *Limiter) (Context, error) {
	return lmt.GlobalLimitReached(ctx, strings.Join(lv, KeyJoinIdentifier))
}

func (lv LimiterKeysValue) PluggableLimits(ctx context.Context, lmt *Limiter) (Context, error) {
	return lmt.PluggableLimitReached(ctx, strings.Join(lv, KeyJoinIdentifier))
}

func (lv LimiterKeysValue) Limits(ctx context.Context, lmt *Limiter) (Context, error) {
	return lmt.LimitReached(ctx, strings.Join(lv, KeyJoinIdentifier))
}
