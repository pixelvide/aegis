package templates

import "fmt"

// VerifyEmail returns the subject and HTML body for an email verification message.
func VerifyEmail(verifyURL string) (subject, body string) {
	subject = "Aegis — Verify your email address"

	content := fmt.Sprintf(`%s
              %s
              %s
              %s`,
		Heading("Verify your email address"),
		Paragraph("Please confirm your email address by clicking the button below. This link expires in <strong>24 hours</strong>."),
		Button(verifyURL, "Verify Email Address"),
		Muted("If you didn't create an Aegis account or add this email, you can safely ignore this message."),
	)

	body = BaseLayout("Verify your email for Aegis", content)
	return
}
