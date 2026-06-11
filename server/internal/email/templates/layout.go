// Package templates provides HTML email templates for Aegis transactional emails.
//
// All templates use a shared base layout with consistent branding.
// Each template function returns (subject, body) so subject lines are
// co-located with their content.
package templates

import (
	"fmt"
	"strings"
)

// BaseLayout wraps email content in a consistent branded layout.
// preheader is the hidden inbox preview text. bodyHTML is the main content.
func BaseLayout(preheader, bodyHTML string) string {
	return fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <meta name="viewport" content="width=device-width, initial-scale=1.0">
  <meta http-equiv="X-UA-Compatible" content="IE=edge">
  <title>Aegis</title>
  <!--[if mso]>
  <noscript>
    <xml>
      <o:OfficeDocumentSettings>
        <o:PixelsPerInch>96</o:PixelsPerInch>
      </o:OfficeDocumentSettings>
    </xml>
  </noscript>
  <![endif]-->
  <style>
    body { margin: 0; padding: 0; -webkit-text-size-adjust: 100%%; -ms-text-size-adjust: 100%%; }
    table { border-collapse: collapse; mso-table-lspace: 0pt; mso-table-rspace: 0pt; }
    img { border: 0; height: auto; line-height: 100%%; outline: none; text-decoration: none; }
  </style>
</head>
<body style="margin:0;padding:0;background-color:#f4f4f5;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,'Helvetica Neue',Arial,sans-serif;">
  <!-- Preheader text (hidden, shows in inbox preview) -->
  <div style="display:none;font-size:1px;color:#f4f4f5;line-height:1px;max-height:0px;max-width:0px;opacity:0;overflow:hidden;">
    %s
  </div>

  <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="background-color:#f4f4f5;padding:32px 0;">
    <tr>
      <td align="center">
        <!-- Logo / Brand -->
        <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:560px;">
          <tr>
            <td style="padding:0 24px 24px 24px;">
              <span style="font-size:20px;font-weight:700;color:#18181b;letter-spacing:-0.5px;">&#128737;&#65039; Aegis</span>
            </td>
          </tr>
        </table>

        <!-- Main Card -->
        <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:560px;background-color:#ffffff;border-radius:8px;overflow:hidden;">
          <tr>
            <td style="padding:32px 32px 24px 32px;">
              %s
            </td>
          </tr>
        </table>

        <!-- Footer -->
        <table role="presentation" width="100%%" cellpadding="0" cellspacing="0" style="max-width:560px;">
          <tr>
            <td style="padding:24px;text-align:center;">
              <p style="margin:0;font-size:12px;color:#a1a1aa;line-height:1.5;">
                Aegis Security Platform<br>
                This is an automated message — please do not reply directly.
              </p>
            </td>
          </tr>
        </table>
      </td>
    </tr>
  </table>
</body>
</html>`, preheader, bodyHTML)
}

// Button renders a CTA button with a plain-text link fallback below it.
// Some email clients strip styled links, so the raw URL is always shown.
func Button(url, label string) string {
	return fmt.Sprintf(`
              <table role="presentation" cellpadding="0" cellspacing="0" style="margin:24px 0;">
                <tr>
                  <td align="center" style="border-radius:6px;background-color:#18181b;">
                    <a href="%s" target="_blank" style="display:inline-block;padding:12px 28px;font-size:14px;font-weight:600;color:#ffffff;text-decoration:none;border-radius:6px;">
                      %s
                    </a>
                  </td>
                </tr>
              </table>
              <p style="margin:0;font-size:12px;color:#a1a1aa;line-height:1.5;word-break:break-all;">
                If the button doesn't work, copy and paste this link into your browser:<br>
                <a href="%s" style="color:#3b82f6;text-decoration:underline;">%s</a>
              </p>`, url, label, url, url)
}

// InfoRow renders a label-value pair for data tables (login details, etc.)
func InfoRow(label, value string) string {
	return fmt.Sprintf(`
                <tr>
                  <td style="padding:6px 0;color:#71717a;font-size:14px;width:140px;vertical-align:top;">%s</td>
                  <td style="padding:6px 0;color:#18181b;font-size:14px;font-weight:500;">%s</td>
                </tr>`, label, value)
}

// Heading renders a styled h2 heading.
func Heading(text string) string {
	return fmt.Sprintf(`<h2 style="margin:0 0 8px 0;font-size:20px;font-weight:600;color:#18181b;">%s</h2>`, text)
}

// Paragraph renders a styled paragraph.
func Paragraph(text string) string {
	return fmt.Sprintf(`<p style="margin:0 0 16px 0;font-size:14px;color:#52525b;line-height:1.6;">%s</p>`, text)
}

// Muted renders a muted/subtle paragraph (for disclaimers).
func Muted(text string) string {
	return fmt.Sprintf(`<p style="margin:16px 0 0 0;font-size:13px;color:#a1a1aa;line-height:1.5;">%s</p>`, text)
}

// WarningBox renders a red warning callout.
func WarningBox(text string) string {
	return fmt.Sprintf(`
              <div style="padding:12px 16px;background-color:#fef2f2;border-left:4px solid #dc2626;border-radius:4px;margin:16px 0 0 0;">
                <p style="margin:0;font-size:13px;color:#991b1b;line-height:1.5;">%s</p>
              </div>`, text)
}

// HtmlEscape escapes HTML special characters to prevent XSS in email templates.
func HtmlEscape(s string) string {
	s = strings.ReplaceAll(s, "&", "&amp;")
	s = strings.ReplaceAll(s, "<", "&lt;")
	s = strings.ReplaceAll(s, ">", "&gt;")
	s = strings.ReplaceAll(s, `"`, "&quot;")
	s = strings.ReplaceAll(s, "'", "&#39;")
	return s
}
