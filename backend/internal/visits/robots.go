// Package visits turns what a redirect knows about a visitor (IP and
// User-Agent) into what is kept: whether it is a robot, the country and the
// day's salted visitor hash (ADR 0007).
package visits

import "strings"

// robotMarkers are lowercase fragments of the User-Agent of link preview
// robots and search crawlers. A bare "bot" is left out on purpose: it would
// also match real phones, such as Cubot models.
var robotMarkers = []string{
	// Link previews of messaging apps and social networks.
	"whatsapp",
	"telegrambot",
	"slackbot",
	"slack-imgproxy",
	"discordbot",
	"facebookexternalhit",
	"facebot",
	"meta-externalagent",
	"twitterbot",
	"linkedinbot",
	"skypeuripreview",
	"pinterestbot",
	"redditbot",
	"embedly",
	"iframely",
	"vkshare",
	// Search crawlers.
	"googlebot",
	"bingbot",
	"duckduckbot",
	"yandexbot",
	"applebot",
	"baiduspider",
}

// IsRobot reports whether the User-Agent belongs to a known robot. Browsers
// always send one, so an empty User-Agent is a script and counts as a robot.
func IsRobot(userAgent string) bool {
	agent := strings.ToLower(strings.TrimSpace(userAgent))
	if agent == "" {
		return true
	}
	for _, marker := range robotMarkers {
		if strings.Contains(agent, marker) {
			return true
		}
	}
	return false
}
