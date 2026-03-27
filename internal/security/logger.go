package security

import "log"

// Security event constants for structured logging.
const (
	EventLoginSuccess          = "LOGIN_SUCCESS"
	EventLoginFailed           = "LOGIN_FAILED"
	EventLoginBlocked          = "LOGIN_BLOCKED"
	EventRegister              = "REGISTER"
	EventRegisterBlocked       = "REGISTER_BLOCKED"
	EventPasswordReset         = "PASSWORD_RESET"
	EventPasswordChange        = "PASSWORD_CHANGE"
	EventForgotPassword        = "FORGOT_PASSWORD"
	EventForgotPasswordBlocked = "FORGOT_PASSWORD_BLOCKED"
	EventFileRejected          = "FILE_REJECTED"
	EventInspectionBlocked     = "INSPECTION_BLOCKED"
)

// Log writes a security event to the application log.
// Format: [SECURITY] event=LOGIN_FAILED ip=1.2.3.4 detail
func Log(event, ip, detail string) {
	if detail != "" {
		log.Printf("[SECURITY] event=%s ip=%s %s", event, ip, detail)
	} else {
		log.Printf("[SECURITY] event=%s ip=%s", event, ip)
	}
}
