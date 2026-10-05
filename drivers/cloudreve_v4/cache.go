package cloudreve_v4

import (
	"net/url"
	"strconv"
	"strings"
	"time"
)

type fileURLCacheDecision struct {
	Expiration     *time.Duration
	RefreshNoCache bool
	Expired        bool
}

type signedURLExpiration struct {
	ExpiresAt time.Time
	// ValidForKnown is true only when the URL exposes its original TTL.
	// It controls whether a nearly-expired long-lived URL is worth refreshing.
	ValidFor      time.Duration
	ValidForKnown bool
}

const (
	fileURLCacheSafeMargin = 10 * time.Minute
	maxSignedURLTTL        = 7 * 24 * time.Hour
	unixTimestampMin       = 1_000_000_000
	unixTimestampMax       = 10_000_000_000
	upyunTokenSignLen      = 8
	// Only refresh long-lived signatures: after keeping a 10-minute safety margin,
	// the refreshed URL should still have at least another 10 minutes worth caching.
	fileURLRefreshMinSignedTTL = fileURLCacheSafeMargin + 10*time.Minute
)

func decideFileURLCache(signedExpires signedURLExpiration, signed bool, crExpires time.Time, allowRefresh bool) fileURLCacheDecision {
	now := time.Now()
	if signed {
		// Signed object storage URLs carry their real expiration in the URL itself.
		// Cloudreve's expires field is a default entity URL TTL, so it must not cap
		// signed URL cache time.
		exp := signedExpires.ExpiresAt.Add(-fileURLCacheSafeMargin).Sub(now)
		if exp > 0 {
			return fileURLCacheDecision{Expiration: &exp}
		}

		if allowRefresh && (!now.Before(signedExpires.ExpiresAt) ||
			(signedExpires.ValidForKnown && signedExpires.ValidFor > fileURLRefreshMinSignedTTL)) {
			return fileURLCacheDecision{RefreshNoCache: true}
		}
		if !now.Before(signedExpires.ExpiresAt) {
			return fileURLCacheDecision{Expired: true}
		}
		return fileURLCacheDecision{}
	}
	exp := crExpires.Sub(now) * 3 / 4
	if exp > 0 {
		return fileURLCacheDecision{Expiration: &exp}
	}
	return fileURLCacheDecision{}
}

// parseObjectStorageSignedURLExpiration extracts expiration from URL-level
// signatures. It covers Cloudreve proxy links plus common object storage
// providers (S3/KS3, OSS, COS, Qiniu, USS/Upyun, OBS-style Expires). Public or
// provider URLs without signature metadata intentionally return false so the
// caller can fall back to Cloudreve's API expires field.
func parseObjectStorageSignedURLExpiration(rawURL string) (signedURLExpiration, bool) {
	u, err := url.Parse(rawURL)
	if err != nil {
		return signedURLExpiration{}, false
	}

	q := u.Query()
	parsers := []func(url.Values) (signedURLExpiration, bool){
		parseCloudreveSignedURLExpiration,
		parseS3SignedURLExpiration,
		parseOSSSignedURLExpiration,
		parseCOSSignTimeExpiration,
		parseQiniuSignedURLExpiration,
		parseUpyunSignedURLExpiration,
		parseUnixExpiresSignedURLExpiration,
	}
	for _, parser := range parsers {
		if exp, ok := parser(q); ok {
			return exp, true
		}
	}
	return signedURLExpiration{}, false
}

func parseCloudreveSignedURLExpiration(q url.Values) (signedURLExpiration, bool) {
	sign := q.Get("sign")
	if sign == "" {
		return signedURLExpiration{}, false
	}
	parts := strings.Split(sign, ":")
	if len(parts) != 2 || parts[0] == "" {
		return signedURLExpiration{}, false
	}
	return parseUnixExpiration(parts[len(parts)-1])
}

func parseS3SignedURLExpiration(q url.Values) (signedURLExpiration, bool) {
	if q.Get("X-Amz-Algorithm") != "AWS4-HMAC-SHA256" ||
		q.Get("X-Amz-Credential") == "" ||
		q.Get("X-Amz-Signature") == "" {
		return signedURLExpiration{}, false
	}
	return parseDateAndTTLExpiration(q.Get("X-Amz-Date"), q.Get("X-Amz-Expires"), "20060102T150405Z")
}

func parseOSSSignedURLExpiration(q url.Values) (signedURLExpiration, bool) {
	if q.Get("x-oss-signature-version") != "OSS4-HMAC-SHA256" ||
		q.Get("x-oss-credential") == "" ||
		q.Get("x-oss-signature") == "" {
		return signedURLExpiration{}, false
	}
	return parseDateAndTTLExpiration(q.Get("x-oss-date"), q.Get("x-oss-expires"), "20060102T150405Z")
}

func parseCOSSignTimeExpiration(q url.Values) (signedURLExpiration, bool) {
	if q.Get("q-signature") == "" ||
		q.Get("q-ak") == "" ||
		q.Get("q-key-time") == "" {
		return signedURLExpiration{}, false
	}
	signTime := q.Get("q-sign-time")
	if signTime == "" {
		return signedURLExpiration{}, false
	}
	parts := strings.Split(signTime, ";")
	if len(parts) != 2 {
		return signedURLExpiration{}, false
	}
	start, err := strconv.ParseInt(parts[0], 10, 64)
	if err != nil {
		return signedURLExpiration{}, false
	}
	end, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil || end <= start {
		return signedURLExpiration{}, false
	}
	validForSeconds := end - start
	return signedURLExpiration{
		ExpiresAt:     time.Unix(end, 0),
		ValidFor:      time.Duration(validForSeconds) * time.Second,
		ValidForKnown: true,
	}, true
}

func parseQiniuSignedURLExpiration(q url.Values) (signedURLExpiration, bool) {
	tokenParts := strings.Split(q.Get("token"), ":")
	if len(tokenParts) != 2 || tokenParts[0] == "" || tokenParts[1] == "" {
		return signedURLExpiration{}, false
	}
	return parseUnixExpiration(q.Get("e"))
}

func parseUpyunSignedURLExpiration(q url.Values) (signedURLExpiration, bool) {
	upt := q.Get("_upt")
	if len(upt) <= upyunTokenSignLen {
		return signedURLExpiration{}, false
	}
	if _, err := strconv.ParseUint(upt[:upyunTokenSignLen], 16, 32); err != nil {
		return signedURLExpiration{}, false
	}
	return parseUnixExpiration(upt[upyunTokenSignLen:])
}

func parseUnixExpiresSignedURLExpiration(q url.Values) (signedURLExpiration, bool) {
	if q.Get("Signature") == "" ||
		(q.Get("AWSAccessKeyId") == "" &&
			q.Get("AccessKeyId") == "" &&
			q.Get("OSSAccessKeyId") == "" &&
			q.Get("KSSAccessKeyId") == "") {
		return signedURLExpiration{}, false
	}
	return parseUnixExpiration(q.Get("Expires"))
}

func parseDateAndTTLExpiration(date string, ttl string, layout string) (signedURLExpiration, bool) {
	if date == "" || ttl == "" {
		return signedURLExpiration{}, false
	}
	signedAt, err := time.Parse(layout, date)
	if err != nil {
		return signedURLExpiration{}, false
	}
	expiresSeconds, err := strconv.ParseInt(ttl, 10, 64)
	if err != nil || expiresSeconds <= 0 {
		return signedURLExpiration{}, false
	}
	if expiresSeconds > int64(maxSignedURLTTL/time.Second) {
		return signedURLExpiration{}, false
	}

	validFor := time.Duration(expiresSeconds) * time.Second
	return signedURLExpiration{
		ExpiresAt:     signedAt.Add(validFor),
		ValidFor:      validFor,
		ValidForKnown: true,
	}, true
}

func parseUnixExpiration(expire string) (signedURLExpiration, bool) {
	if expire == "" {
		return signedURLExpiration{}, false
	}
	expiresAt, err := strconv.ParseInt(expire, 10, 64)
	if err != nil || expiresAt <= unixTimestampMin || expiresAt >= unixTimestampMax {
		return signedURLExpiration{}, false
	}
	return signedURLExpiration{ExpiresAt: time.Unix(expiresAt, 0)}, true
}
