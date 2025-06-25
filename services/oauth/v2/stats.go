package v2

import (
	"strconv"
	"strings"
	"time"

	"github.com/samber/lo"

	"github.com/rudderlabs/rudder-go-kit/stats"
	"github.com/rudderlabs/rudder-server/services/oauth/v2/common"
)

const OAUTH_V2_STAT_PREFIX = "oauth_action"

type OAuthStats struct {
	stats           stats.Stats
	accountId       string // destinationId -> for action == auth_status_inactive, accountId -> for action == refresh_token/fetch_token
	destinationId   string
	workspaceID     string
	errorMessage    string
	rudderCategory  string // destination
	statName        string
	isCallToCpApi   bool   // is a call being made to control-plane APIs
	authErrCategory string // for action=refresh_token -> REFRESH_TOKEN, for action=fetch_token -> "", for action=auth_status_inactive -> auth_status_inactive
	destDefName     string
	flowType        common.RudderFlow // delivery, delete
	action          string            // refresh_token, fetch_token, auth_status_inactive

	// New fields for comprehensive metrics
	workerId        int    // worker ID to track worker behavior patterns
	cacheOperation  string // cache operation type: hit, miss, store, delete
	tokenStatus     string // token status: fresh, stale, expired, invalid
	concurrentCount int    // number of concurrent operations
	refreshReason   string // reason for refresh: expired, stale, missing, error
}

type OAuthStatsHandler struct {
	stats       stats.Stats
	defaultTags stats.Tags
}

func GetDefaultTagsFromOAuthStats(oauthStats *OAuthStats) stats.Tags {
	tags := stats.Tags{
		"accountId":       oauthStats.accountId,
		"destinationId":   oauthStats.destinationId,
		"workspaceId":     oauthStats.workspaceID,
		"rudderCategory":  "destination",
		"isCallToCpApi":   strconv.FormatBool(oauthStats.isCallToCpApi),
		"authErrCategory": oauthStats.authErrCategory,
		"destType":        oauthStats.destDefName,
		"flowType":        string(oauthStats.flowType),
		"action":          oauthStats.action,
		"oauthVersion":    "v2",
	}

	// Add new metric tags if they have values
	if oauthStats.workerId > 0 {
		tags["workerId"] = strconv.Itoa(oauthStats.workerId)
	}
	if oauthStats.cacheOperation != "" {
		tags["cacheOperation"] = oauthStats.cacheOperation
	}
	if oauthStats.tokenStatus != "" {
		tags["tokenStatus"] = oauthStats.tokenStatus
	}
	if oauthStats.concurrentCount > 0 {
		tags["concurrentCount"] = strconv.Itoa(oauthStats.concurrentCount)
	}
	if oauthStats.refreshReason != "" {
		tags["refreshReason"] = oauthStats.refreshReason
	}

	return tags
}

func NewStatsHandlerFromOAuthStats(oauthStats *OAuthStats) OAuthStatsHandler {
	defaultTags := GetDefaultTagsFromOAuthStats(oauthStats)
	return OAuthStatsHandler{
		stats:       oauthStats.stats,
		defaultTags: defaultTags,
	}
}

func (m *OAuthStatsHandler) Increment(statSuffix string, tags stats.Tags) {
	statName := strings.Join([]string{OAUTH_V2_STAT_PREFIX, statSuffix}, "_")
	allTags := lo.Assign(m.defaultTags, tags)
	m.stats.NewTaggedStat(statName, stats.CountType, allTags).Increment()
}

func (m *OAuthStatsHandler) SendTiming(startTime time.Time, statSuffix string, tags stats.Tags) {
	statName := strings.Join([]string{OAUTH_V2_STAT_PREFIX, statSuffix}, "_")
	allTags := lo.Assign(m.defaultTags, tags)
	m.stats.NewTaggedStat(statName, stats.TimerType, allTags).SendTiming(time.Since(startTime))
}
