package stripe

type AuthMethod string

const (
	AuthMethodAPIKey   AuthMethod = "API_KEY"
	AuthMethodLocalCLI AuthMethod = "LOCAL_CLI"
)

type Config struct {
	AuthMethod    AuthMethod `json:"auth_method,omitempty"`
	AccountID     string     `json:"account_id,omitempty"     validate:"omitempty,startswith=acct_,max=128"`
	APIKey        string     `json:"api_key,omitempty"`
	WebhookSecret string     `json:"webhook_secret,omitempty"`
	ReturnURL     string     `json:"return_url,omitempty"     validate:"omitempty,url,max=2048"`
}
