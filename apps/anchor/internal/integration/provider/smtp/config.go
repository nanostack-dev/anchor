package smtp

// Encryption identifies the transport encryption mode used to connect to the
// SMTP server. Most modern providers (SendGrid, Mailgun, Proton, AWS SES)
// require either STARTTLS on 587 or implicit TLS on 465.
type Encryption string

const (
	// EncryptionStartTLS upgrades a plaintext connection to TLS via the
	// STARTTLS command. This is the most common and the recommended default
	// (port 587). Supported by all major providers.
	EncryptionStartTLS Encryption = "STARTTLS"
	// EncryptionTLS opens an implicit-TLS connection from the first byte
	// (typically port 465).
	EncryptionTLS Encryption = "TLS"
	// EncryptionNone is plaintext SMTP. Reserved for trusted local relays
	// during development; never use for public submission.
	EncryptionNone Encryption = "NONE"
)

// AuthMethod identifies the SASL mechanism advertised in EHLO. PLAIN and LOGIN
// cover every major hosted provider; we keep the option as part of the config
// so customers running unusual relays can override.
type AuthMethod string

const (
	AuthMethodPlain AuthMethod = "PLAIN"
	AuthMethodLogin AuthMethod = "LOGIN"
)

// Config is the SMTP integration's stored configuration. Values are
// persisted in integration_instances.config_json. Sensitive fields are
// encrypted at rest by PrepareConfigForStorage.
//
// Provider compatibility (verified): SendGrid (smtp.sendgrid.net, user
// "apikey"), Mailgun (smtp.mailgun.org / smtp.eu.mailgun.org, domain SMTP
// credentials), Proton (smtp.protonmail.ch, SMTP token), AWS SES
// (email-smtp.<region>.amazonaws.com, IAM-derived SMTP credentials). All four
// accept SMTP-AUTH PLAIN/LOGIN over STARTTLS on port 587.
type Config struct {
	Host        string     `json:"host"`
	Port        int        `json:"port"`
	Encryption  Encryption `json:"encryption"`
	AuthMethod  AuthMethod `json:"auth_method"`
	Username    string     `json:"username"`
	Password    string     `json:"password,omitempty"`
	FromAddress string     `json:"from_address"`
	FromName    string     `json:"from_name,omitempty"`
	ReplyTo     string     `json:"reply_to,omitempty"`
}
