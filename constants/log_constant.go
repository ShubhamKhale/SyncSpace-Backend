package constants

// Log tag constants prefix every log line with the originating component,
// making it easy to filter logs in production.
const (
	LogTagMain   = "[MAIN]"
	LogTagDB     = "[DB]"
	LogTagBoard  = "[BOARD]"
	LogTagHealth = "[HEALTH]"
	LogTagAuth   = "[AUTH]"
	LogTagCrypto = "[CRYPTO]"
	LogTagUser      = "[USER]"
	LogTagOrg       = "[ORG]"
	LogTagTask      = "[TASK]"
	LogTagDashboard    = "[DASHBOARD]"
	LogTagActivity     = "[ACTIVITY]"
	LogTagAnalytics    = "[ANALYTICS]"
	LogTagNotification  = "[NOTIFICATION]"
	LogTagLinkedResource = "[LINKED_RESOURCE]"
	LogTagFlow           = "[FLOW]"
	LogTagAI             = "[AI]"
)
