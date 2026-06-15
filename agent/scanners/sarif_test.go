package scanners

import (
	"testing"
)

// testSARIF is a minimal valid SARIF v2.1.0 document for testing.
const testSARIF = `{
  "version": "2.1.0",
  "$schema": "https://raw.githubusercontent.com/oasis-tcs/sarif-spec/master/Schemata/sarif-schema-2.1.0.json",
  "runs": [
    {
      "tool": {
        "driver": {
          "name": "semgrep",
          "version": "1.50.0",
          "rules": [
            {
              "id": "python.lang.security.audit.dangerous-system-call",
              "shortDescription": {
                "text": "Dangerous system call with user input"
              },
              "fullDescription": {
                "text": "User input is passed to os.system() which can lead to command injection."
              },
              "help": {
                "text": "Use subprocess.run() with shell=False instead of os.system()."
              },
              "properties": {
                "tags": ["cwe:CWE-78", "owasp:A03:2021"]
              }
            },
            {
              "id": "python.lang.security.audit.sqli",
              "shortDescription": {
                "text": "SQL injection via string concatenation"
              },
              "fullDescription": {
                "text": "SQL query built using string formatting with user-controlled input."
              },
              "help": {
                "text": "Use parameterized queries."
              },
              "properties": {
                "tags": ["cwe:CWE-89"]
              }
            }
          ]
        }
      },
      "results": [
        {
          "ruleId": "python.lang.security.audit.dangerous-system-call",
          "ruleIndex": 0,
          "level": "error",
          "message": {
            "text": "Dangerous call to os.system() with user input"
          },
          "locations": [
            {
              "physicalLocation": {
                "artifactLocation": {
                  "uri": "file:///workspace/app/utils.py"
                },
                "region": {
                  "startLine": 42,
                  "endLine": 42,
                  "snippet": {
                    "text": "os.system(\"ping \" + user_input)"
                  }
                }
              }
            }
          ]
        },
        {
          "ruleId": "python.lang.security.audit.sqli",
          "ruleIndex": 1,
          "level": "warning",
          "message": {
            "text": "Possible SQL injection"
          },
          "locations": [
            {
              "physicalLocation": {
                "artifactLocation": {
                  "uri": "app/db.py"
                },
                "region": {
                  "startLine": 15,
                  "endLine": 17,
                  "snippet": {
                    "text": "cursor.execute(\"SELECT * FROM users WHERE id=\" + user_id)"
                  }
                }
              }
            }
          ]
        }
      ]
    }
  ]
}`

func TestParseSARIF_ValidDocument(t *testing.T) {
	findings, err := ParseSARIF([]byte(testSARIF), "/workspace")
	if err != nil {
		t.Fatalf("ParseSARIF failed: %v", err)
	}

	if len(findings) != 2 {
		t.Fatalf("expected 2 findings, got %d", len(findings))
	}

	// Check first finding (command injection).
	f1 := findings[0]
	if f1.Title != "Dangerous system call with user input" {
		t.Errorf("f1 title = %q, want 'Dangerous system call with user input'", f1.Title)
	}
	if f1.Severity != "high" {
		t.Errorf("f1 severity = %q, want 'high' (mapped from SARIF 'error')", f1.Severity)
	}
	if f1.CWE != "CWE-78" {
		t.Errorf("f1 CWE = %q, want 'CWE-78'", f1.CWE)
	}
	if f1.FilePath != "app/utils.py" {
		t.Errorf("f1 file = %q, want 'app/utils.py'", f1.FilePath)
	}
	if f1.StartLine != 42 {
		t.Errorf("f1 start_line = %d, want 42", f1.StartLine)
	}
	if f1.VulnerableCode != `os.system("ping " + user_input)` {
		t.Errorf("f1 vulnerable_code = %q", f1.VulnerableCode)
	}
	if f1.Remediation != "Use subprocess.run() with shell=False instead of os.system()." {
		t.Errorf("f1 remediation = %q", f1.Remediation)
	}

	// Check second finding (SQL injection).
	f2 := findings[1]
	if f2.Title != "SQL injection via string concatenation" {
		t.Errorf("f2 title = %q, want 'SQL injection via string concatenation'", f2.Title)
	}
	if f2.Severity != "medium" {
		t.Errorf("f2 severity = %q, want 'medium' (mapped from SARIF 'warning')", f2.Severity)
	}
	if f2.CWE != "CWE-89" {
		t.Errorf("f2 CWE = %q, want 'CWE-89'", f2.CWE)
	}
	if f2.StartLine != 15 || f2.EndLine != 17 {
		t.Errorf("f2 lines = %d-%d, want 15-17", f2.StartLine, f2.EndLine)
	}
}

func TestParseSARIF_Empty(t *testing.T) {
	empty := `{"version": "2.1.0", "runs": [{"tool": {"driver": {"name": "test"}}, "results": []}]}`
	findings, err := ParseSARIF([]byte(empty), "")
	if err != nil {
		t.Fatalf("ParseSARIF failed: %v", err)
	}
	if len(findings) != 0 {
		t.Errorf("expected 0 findings, got %d", len(findings))
	}
}

func TestParseSARIF_InvalidJSON(t *testing.T) {
	_, err := ParseSARIF([]byte("not json"), "")
	if err == nil {
		t.Error("expected error for invalid JSON")
	}
}

func TestMapSARIFLevel(t *testing.T) {
	tests := map[string]string{
		"error":   "high",
		"warning": "medium",
		"note":    "low",
		"none":    "info",
		"ERROR":   "high",
		"":        "medium",
	}
	for input, expected := range tests {
		got := mapSARIFLevel(input)
		if got != expected {
			t.Errorf("mapSARIFLevel(%q) = %q, want %q", input, got, expected)
		}
	}
}

func TestNormalizeURI(t *testing.T) {
	tests := []struct {
		uri       string
		workspace string
		want      string
	}{
		{"file:///workspace/app/main.go", "/workspace", "app/main.go"},
		{"app/main.go", "/workspace", "app/main.go"},
		{"file:///home/user/project/src/lib.rs", "/home/user/project", "src/lib.rs"},
		{"/absolute/path/file.py", "", "absolute/path/file.py"},
	}

	for _, tt := range tests {
		got := normalizeURI(tt.uri, tt.workspace)
		if got != tt.want {
			t.Errorf("normalizeURI(%q, %q) = %q, want %q", tt.uri, tt.workspace, got, tt.want)
		}
	}
}

func TestExtractCWE_Tags(t *testing.T) {
	// Semgrep style: CWE in tags array.
	props := map[string]interface{}{
		"tags": []interface{}{"cwe:CWE-79", "owasp:A03:2021"},
	}
	got := extractCWE("rule1", props)
	if got != "CWE-79" {
		t.Errorf("extractCWE (semgrep tags) = %q, want 'CWE-79'", got)
	}
}

func TestExtractCWE_Direct(t *testing.T) {
	// CodeQL style: CWE as direct property.
	props := map[string]interface{}{
		"cwe": "CWE-89",
	}
	got := extractCWE("rule1", props)
	if got != "CWE-89" {
		t.Errorf("extractCWE (codeql direct) = %q, want 'CWE-89'", got)
	}
}

func TestExtractCWE_Array(t *testing.T) {
	// CodeQL array style.
	props := map[string]interface{}{
		"cwe": []interface{}{"CWE-22", "CWE-23"},
	}
	got := extractCWE("rule1", props)
	if got != "CWE-22" {
		t.Errorf("extractCWE (array) = %q, want 'CWE-22'", got)
	}
}

func TestExtractCWE_BareNumber(t *testing.T) {
	props := map[string]interface{}{
		"tags": []interface{}{"cwe:89"},
	}
	got := extractCWE("rule1", props)
	if got != "CWE-89" {
		t.Errorf("extractCWE (bare number) = %q, want 'CWE-89'", got)
	}
}

func TestExtractCWE_NilProperties(t *testing.T) {
	got := extractCWE("rule1", nil)
	if got != "" {
		t.Errorf("extractCWE (nil) = %q, want empty", got)
	}
}

func TestRegistry_Available(t *testing.T) {
	r := NewRegistry()

	// All scanners should be registered regardless of availability.
	all := r.All()
	if len(all) < 5 {
		t.Errorf("expected at least 5 registered scanners, got %d", len(all))
	}

	// ListInfo should not panic and should contain scanner names.
	info := r.ListInfo()
	if info == "" {
		t.Error("ListInfo returned empty string")
	}
}
