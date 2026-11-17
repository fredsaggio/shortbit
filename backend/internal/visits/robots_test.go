package visits_test

import (
	"testing"

	"github.com/fredsaggio/url-shortener/internal/visits"
)

func TestIsRobot(t *testing.T) {
	tests := []struct {
		name      string
		userAgent string
		want      bool
	}{
		{"empty", "", false},
		{"curl", "curl/8.11.1", false},
		{"WhatsApp preview", "WhatsApp/2.23.20.0 A", true},
		{"Telegram preview", "TelegramBot (like TwitterBot)", true},
		{"Slack preview", "Slackbot-LinkExpanding 1.0 (+https://api.slack.com/robots)", true},
		{"Discord preview", "Mozilla/5.0 (compatible; Discordbot/2.0; +https://discordapp.com)", true},
		{"Facebook preview", "facebookexternalhit/1.1 (+http://www.facebook.com/externalhit_uatext.php)", true},
		{"iMessage preview", "Mozilla/5.0 (Macintosh; Intel Mac OS X 10_11_1) AppleWebKit/601.2.4 (KHTML, like Gecko) Version/9.0.1 Safari/601.2.4 facebookexternalhit/1.1 Facebot Twitterbot/1.0", true},
		{"Google crawler", "Mozilla/5.0 (compatible; Googlebot/2.1; +http://www.google.com/bot.html)", true},
		{"Chrome on Android", "Mozilla/5.0 (Linux; Android 14; Pixel 8) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36", false},
		{"Safari on iPhone", "Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Version/18.1 Mobile/15E148 Safari/604.1", false},
		{"Cubot phone", "Mozilla/5.0 (Linux; Android 10; CUBOT X30) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/120.0.0.0 Mobile Safari/537.36", false},
		{"Facebook in-app browser", "Mozilla/5.0 (iPhone; CPU iPhone OS 18_1 like Mac OS X) AppleWebKit/605.1.15 (KHTML, like Gecko) Mobile/15E148 [FBAN/FBIOS;FBAV/490.0.0.0]", false},
		{"Instagram in-app browser", "Mozilla/5.0 (Linux; Android 14; SM-S918B) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/131.0.0.0 Mobile Safari/537.36 Instagram 350.0.0.0", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := visits.IsRobot(tt.userAgent); got != tt.want {
				t.Errorf("IsRobot(%q) = %v, want %v", tt.userAgent, got, tt.want)
			}
		})
	}
}
