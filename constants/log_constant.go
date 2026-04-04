package constants

// Log tag constants prefix every log line with the originating component,
// making it easy to filter logs in production.
const (
	LogTagMain   = "[MAIN]"
	LogTagDB     = "[DB]"
	LogTagBoard  = "[BOARD]"
	LogTagHealth = "[HEALTH]"
)
