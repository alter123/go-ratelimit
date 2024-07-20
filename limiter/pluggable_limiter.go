package limiter

import (
	"context"
	"strings"

	limiterlib "github.com/ulule/limiter/v3"
)

type PluggableLimiter []Pluggable

// PluggableLimiter provides interface to generate overridable limiters
type Pluggable struct {
	// Maximum number of requests to limit per ttl
	E ExpirableOptions
	L *limiterlib.Limiter
}

func NewPluggableLimiter(eo []ExpirableOptions) *PluggableLimiter {
	pl := make(PluggableLimiter, len(eo))
	for i, e := range eo {
		ll := &limiterlib.Limiter{
			Store: redisStore,
			Rate: limiterlib.Rate{
				Period: e.DefaultExpirationTTL,
				Limit:  e.ExpireJobInterval,
			},
		}

		pl[i] = Pluggable{
			E: e,
			L: ll,
		}
	}
	return &pl
}

func (l *Limiter) IsPluggableLimiterValid() (isValid bool) {
	if l.pluggableLimiter == nil {
		return
	}

	for _, p := range *l.pluggableLimiter {
		if p.L.Store == nil {
			continue
		}
		if p.E.Suffix == "" {
			continue
		}
		isValid = true
	}
	return
}

func (l *Limiter) PluggableLimitReached(ctx context.Context, key string) (lctx Context, err error) {
	if !l.IsPluggableLimiterValid() {
		return Context{}, nil
	}

	for _, p := range *l.pluggableLimiter {
		lctx, err = l.pluggableLimiterValidator(ctx, &p, key)
		if err != nil {
			return
		}
		if lctx.LimitReached() {
			return
		}
	}
	return
}

func (l *Limiter) pluggableLimiterValidator(ctx context.Context, p *Pluggable, key string) (lctx Context, err error) {
	if p.E.Suffix == "" {
		return
	}
	if p.L.Store == nil {
		return
	}
	lctxi, lerr := p.L.Get(ctx, strings.Join([]string{key, p.E.Suffix}, KeyJoinIdentifier))
	if lerr != nil {
		return Context{}, lerr
	}
	lctx = Context(lctxi)
	return
}
