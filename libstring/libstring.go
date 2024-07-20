// Package libstring provides various string related functions.
package libstring

import (
	"net"
	"net/http"
	"strings"
)

// RemoteIP finds IP Address given http.Request struct
func RemoteIP(ipLookups []string, forwardedForIndexFromBehind int, r *http.Request) string {
	cfConnectingIP := r.Header.Get("CF-Connecting-IP")
	realIP := r.Header.Get("X-Real-IP")
	forwardedFor := r.Header.Get("X-Forwarded-For")

	for _, lookup := range ipLookups {
		if lookup == "CF-Connecting-IP" && cfConnectingIP != "" {
			return cfConnectingIP
		}

		if lookup == "RemoteAddr" {
			// 1. Cover the basic use cases for both ipv4 and ipv6
			ip, _, err := net.SplitHostPort(r.RemoteAddr)
			if err != nil {
				// 2. Upon error, just return the remote addr.
				return r.RemoteAddr
			}
			return ip
		}
		if lookup == "X-Forwarded-For" && forwardedFor != "" {
			// X-Forwarded-For is potentially a list of addresses separated with ","
			parts := strings.Split(forwardedFor, ",")
			for i, p := range parts {
				parts[i] = strings.TrimSpace(p)
			}

			partIndex := len(parts) - 1 - forwardedForIndexFromBehind
			if partIndex < 0 {
				partIndex = 0
			}

			return parts[partIndex]
		}
		if lookup == "X-Real-IP" && realIP != "" {
			return realIP
		}
	}

	return ""
}
