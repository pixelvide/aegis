package templates

import "fmt"

// MFACode returns the subject and HTML body for an MFA one-time code email.
func MFACode(code string) (subject, body string) {
	subject = "Your Aegis verification code"

	escapedCode := HtmlEscape(code)

	content := fmt.Sprintf(`%s
              %s

              <div style="text-align:center;margin:0 0 24px 0;">
                <div style="display:inline-block;padding:16px 32px;background-color:#f4f4f5;border-radius:8px;border:1px solid #e4e4e7;">
                  <span style="font-size:32px;font-weight:700;letter-spacing:8px;color:#18181b;font-family:'Courier New',monospace;">%s</span>
                </div>
              </div>

              %s
              %s`,
		Heading("Your verification code"),
		Paragraph("Use the following code to complete your sign-in. This code expires in <strong>5 minutes</strong>."),
		escapedCode,
		Paragraph("Enter this code on the verification page to continue."),
		Muted("If you didn't try to sign in, someone may be trying to access your account. Please change your password immediately."),
	)

	body = BaseLayout(fmt.Sprintf("Your Aegis verification code is %s", escapedCode), content)
	return
}
