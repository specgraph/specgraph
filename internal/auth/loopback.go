// SPDX-License-Identifier: Apache-2.0
// Copyright 2026 Sean Brandt

package auth

// IsLiteralLoopbackHost reports whether host is exactly the IPv4 or IPv6
// loopback literal. It deliberately rejects "localhost" (resolver/DNS-rebinding
// risk, RFC 8252 §8.3) and any non-canonical form. Callers pass the result of
// net/url's url.Hostname(), which strips the brackets from "[::1]" → "::1".
func IsLiteralLoopbackHost(host string) bool {
	return host == "127.0.0.1" || host == "::1"
}
