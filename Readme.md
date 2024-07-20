# Go Ratelimit

_Request level utility wrapper for [github.com/ulule/limiter](https://github.com/ulule/limiter)._

- Limit for a particular route
- Custom limit rules per route
- Limit requests based on headers, cookies or any context present in the request (or plug custom ml models)
- Supports optional global limiting, additionally
- Graceful degradation
- Tied to Redis, for now

## Setup & Usage

_Using [HTTP StdLib](https://pkg.go.dev/net/http)_

- Install package using [Go Modules](https://github.com/golang/go/wiki/Modules)

```bash
$ go get github.com/alter123/go-ratelimit
```

- Globally initialize the limiter by providing Redis config (& other optional config)

```go
func RateLimiterInit(config RateLimitConfig) {
	goratelimit.Init(limiter.LimiterOptions{
		Redis: *redis.Options,
        // global params to process (e.g. limit on user id, instead of ipAddr)
		FuncFetchFromContext: limiter.FuncFetchParamFromContext,
        // request level params (based on query params etc, passed via `SetAdditionalContextParam`)
		FuncAdditionalContext: limiter.FuncFetchParamFromContext,
	})
}
```

- Create middleware implementation (based upon the framework)

```go
func LimitHandler(limiter *limiter.Limiter) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
        if lmtCtx, err := goratelimit.LimitByRequest(limiter, r); err == nil && lmtCtx.Reached {
            http.Error(w, "Limit exceeded", http.StatusTooManyRequests)
			return
		} else if err != nil {
			// log error, returned from the limiter
		}

		h.ServeHTTP(w, r)
	})
}
```

- Plug limiter config into route

```go
rateLimitHandler := LimitHandler(goratelimit.NewLimiter(2, 10*time.Second))

http.Handle("/", rateLimitHandler(http.HandlerFunc(index)))
```

## Contributing

- Running tests

```bash
$ go test
```

- Report issues at [GitHub issue tracker](https://github.com/alter123/go-ratelimit/issues/new)

- To add a new feature, create a new [GitHub pull request](https://github.com/alter123/go-ratelimit/compare)