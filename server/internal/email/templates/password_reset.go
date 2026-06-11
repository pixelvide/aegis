package templates

import "fmt"

// PasswordReset returns the subject and HTML body for a password reset email.
func PasswordReset(resetURL string) (subject, body string) {
	subject = "Aegis — Reset your password"

	content := fmt.Sprintf(`%s
              %s
              %s
              %s
              %s`,
		Heading("Reset your password"),
		Paragraph("We received a request to reset the password for your Aegis account. Click the button below to choose a new password."),
		Paragraph("This link expires in <strong>1 hour</strong>."),
		Button(resetURL, "Reset Password"),
		Muted("If you didn't request a password reset, you can safely ignore this email. Your password will not be changed."),
	)

	body = BaseLayout("Reset your Aegis password", content)
	return
}
