package templates

import "fmt"

// PasswordChanged returns the subject and HTML body for a password change confirmation.
func PasswordChanged(name string) (subject, body string) {
	subject = "Aegis — Your password was changed"

	content := fmt.Sprintf(`%s
              %s
              %s
              %s`,
		Heading("Password changed"),
		Paragraph(fmt.Sprintf("Hi %s, your Aegis account password was successfully changed.", HtmlEscape(name))),
		Paragraph("If you made this change, no further action is needed."),
		WarningBox("<strong>Didn't change your password?</strong> Your account may be compromised. Reset your password immediately and review your active sessions."),
	)

	body = BaseLayout("Your Aegis password was changed", content)
	return
}
