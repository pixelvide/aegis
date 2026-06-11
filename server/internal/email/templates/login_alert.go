package templates

import "fmt"

// LoginAlert returns the subject and HTML body for a new sign-in notification email.
func LoginAlert(name, ip, browser, os, deviceType, loginTime string) (subject, body string) {
	subject = "Aegis — New sign-in to your account"

	content := fmt.Sprintf(`%s
              %s

              <table role="presentation" cellpadding="0" cellspacing="0" style="width:100%%;background-color:#fafafa;border-radius:6px;margin:0 0 20px 0;">
                <tr><td style="padding:16px;">
                  <table role="presentation" cellpadding="0" cellspacing="0" style="width:100%%;">
                    %s%s%s%s%s
                  </table>
                </td></tr>
              </table>

              %s
              %s`,
		Heading("New sign-in detected"),
		Paragraph(fmt.Sprintf("Hi %s, we noticed a new sign-in to your Aegis account.", HtmlEscape(name))),
		InfoRow("Browser", HtmlEscape(browser)),
		InfoRow("Operating System", HtmlEscape(os)),
		InfoRow("Device", HtmlEscape(deviceType)),
		InfoRow("IP Address", HtmlEscape(ip)),
		InfoRow("Time", HtmlEscape(loginTime)),
		Paragraph("If this was you, no action is needed."),
		WarningBox(fmt.Sprintf("<strong>Don't recognize this activity?</strong> Change your password immediately and review your active sessions in Settings.")),
	)

	body = BaseLayout("New sign-in to your Aegis account", content)
	return
}
