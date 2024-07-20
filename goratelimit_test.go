package goratelimit_test

import (
	"math/rand"
	"net/http"
	"os"
	"strings"
	"testing"
	"time"

	mockredis "github.com/alicebob/miniredis/v2"
	goratelimit "github.com/alter123/go-ratelimit"
	"github.com/alter123/go-ratelimit/limiter"
	"github.com/redis/go-redis/v9"
)

const IPv6Addr = "2001:0db8:85a3:0000:0000:8a2e:0370:7334"

var (
	IsUserIdValid       = true
	IsAdditionalContext = false
)

func generateMockId(n int) string {
	var hexChars = []rune("abcdefABCDEF0123456789")

	b := make([]rune, n)
	for i := range b {
		b[i] = hexChars[rand.Intn(len(hexChars))]
	}
	return string(b)
}

func TestMain(m *testing.M) {
	// create mock redis connection
	s, err := mockredis.Run()
	if err != nil {
		panic(err)
	}
	defer s.Close()

	// Initialise go-ratelimit
	goratelimit.Init(limiter.LimiterOptions{
		Redis: &redis.Options{
			Addr: s.Addr(),
		},
		FuncFetchFromContext: func(_ *http.Request) (string, error) {
			if IsUserIdValid {
				return generateMockId(24), nil
			}
			return "", nil
		},
		FuncAdditionalContext: func(_ *http.Request, _ []string) (string, error) {
			if IsAdditionalContext {
				return generateMockId(5), nil
			}

			return "", nil
		},
		Prefix: "tbrl",
	})

	exitCode := m.Run()
	os.Exit(exitCode)
}

// Test global request without userId been set
func TestSliceKeys(t *testing.T) {

	lmt := goratelimit.NewLimiter(2, 5*time.Minute).SetGlobalLimits(1).
		SetGlobalTtl(5 * time.Second)

	r, err := http.NewRequest("GET", "/", strings.NewReader("!!!"))
	if err != nil {
		t.Fatal(err)
	}

	r.Header.Set("CF-Connecting-IP", IPv6Addr)

	IsUserIdValid = false
	sliceKeys := goratelimit.BuildKeys(lmt, r)
	IsUserIdValid = true

	if len(sliceKeys.Global) == 0 || len(sliceKeys.Request) == 0 {
		t.Fatal("Length of sliceKeys is empty.")
	}

	if len(sliceKeys.Global) != 1 || sliceKeys.Global[0] != IPv6Addr {
		t.Fatal("Global key is not IPv6Addr.")
	}

	if len(sliceKeys.Request) != 5 || sliceKeys.Request[0] != IPv6Addr ||
		sliceKeys.Request[1] != "/" || sliceKeys.Request[2] != "GET" {
		t.Fatal("Request key is not IPv6Addr// /GET.")
	}
}

func TestUserIdKeyLimit(t *testing.T) {
	lmt := goratelimit.NewLimiter(2, 5*time.Minute).SetGlobalLimits(1).
		SetGlobalTtl(5 * time.Second).SetIncludeUserId(true)

	r, err := http.NewRequest("GET", "/", strings.NewReader("!!!"))
	if err != nil {
		t.Fatal(err)
	}

	r.Header.Set("CF-Connecting-IP", IPv6Addr)

	sliceKeys := goratelimit.BuildKeys(lmt, r)
	if len(sliceKeys.Global) != 0 {
		t.Fatal("Global key is not empty.")
	}

	if len(sliceKeys.Request) != 5 || sliceKeys.Request[0] != IPv6Addr ||
		sliceKeys.Request[1] != "/" || sliceKeys.Request[2] != "GET" ||
		len(sliceKeys.Request[3]) != 24 {
		t.Fatal("Request key is not IPv6Addr// /GET.")
	}
}

func TestGlobalLimits(t *testing.T) {
	s, err := mockredis.Run()
	if err != nil {
		panic(err)
	}
	defer s.Close()

	lmt := goratelimit.NewLimiter(2, 5*time.Second).SetIncludeUserId(false)
	request := func() (limiter.Context, error) {
		r, err := http.NewRequest("GET", "/", strings.NewReader("!!!"))
		if err != nil {
			t.Fatal(err)
		}

		r.Header.Set("CF-Connecting-IP", IPv6Addr)
		return goratelimit.LimitByRequest(lmt, r)
	}

	var lmtCtx limiter.Context

	IsAdditionalContext = true

	for i := 0; i < 60; i++ {
		if lmtCtx, err = request(); err != nil {
			t.Fatal(err)
		}
	}

	// blocked for individual request
	if lmtCtx.Limit != 2 || lmtCtx.Remaining != 1 || lmtCtx.Reached || lmtCtx.Reset == 0 {
		t.Fatal("Limit is not 1.")
	}

	if lmtCtx, err = request(); err != nil {
		t.Fatal(err)
	}
	// should be blocked by global limiter
	if lmtCtx.Limit != 60 || lmtCtx.Remaining != 0 || !lmtCtx.Reached || lmtCtx.Reset == 0 {
		t.Fatal("Limit is not 60.")
	}

	IsAdditionalContext = false
}

func TestGlobalLimitsWithoutAdditionalContext(t *testing.T) {
	lmt := goratelimit.NewLimiter(2, 5*time.Second).SetIncludeUserId(false)

	var lmtCtx limiter.Context

	for i := 0; i < 61; i++ {
		r, err := http.NewRequest("GET", generateMockId(5), strings.NewReader("!!!"))
		if err != nil {
			t.Fatal(err)
		}

		r.Header.Set("CF-Connecting-IP", IPv6Addr)
		if lmtCtx, err = goratelimit.LimitByRequest(lmt, r); err != nil {
			t.Fatal(err)
		}
	}

	if lmtCtx.Limit != 60 || lmtCtx.Remaining != 0 || !lmtCtx.Reached || lmtCtx.Reset == 0 {
		t.Fatal("Limit is not 1.")
	}
}

// test when userId & ipAddr is not set
func TestIgnoredRequests(t *testing.T) {
	lmt := goratelimit.NewLimiter(2, 5*time.Second).SetIncludeUserId(false)

	r, err := http.NewRequest("GET", "/", strings.NewReader("!!!"))
	if err != nil {
		t.Fatal(err)
	}

	var lmtCtx limiter.Context
	lmtCtx, err = goratelimit.LimitByRequest(lmt, r)
	if err != nil {
		t.Fatal(err)
	}

	if lmtCtx.Limit != 0 || lmtCtx.Remaining != 0 || lmtCtx.Reached || lmtCtx.Reset != 0 {
		t.Fatal("Limit is not 0.")
	}
}
