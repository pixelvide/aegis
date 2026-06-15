package pipeline

import (
	"strings"
	"testing"
)

func TestChunkFile_SmallFile(t *testing.T) {
	c := NewChunker(150, 3)
	content := `package main

func main() {
	fmt.Println("hello")
}`
	chunks := c.ChunkFile("main.go", content)
	if len(chunks) != 1 {
		t.Fatalf("expected 1 chunk, got %d", len(chunks))
	}
	if chunks[0].Language != "go" {
		t.Errorf("expected language 'go', got %q", chunks[0].Language)
	}
	if chunks[0].FilePath != "main.go" {
		t.Errorf("expected file path 'main.go', got %q", chunks[0].FilePath)
	}
}

func TestChunkFile_MultipleFunctions(t *testing.T) {
	c := NewChunker(30, 3) // Small MaxLines to force multi-chunk output.

	// Build a Go file with 3 functions, each >30 lines total.
	var b strings.Builder
	b.WriteString("package auth\n\nimport \"fmt\"\n\n")
	b.WriteString("func Login(user, pass string) error {\n")
	for i := 0; i < 15; i++ {
		b.WriteString("\tfmt.Println(\"login logic\")\n")
	}
	b.WriteString("}\n\n")
	b.WriteString("func Logout() {\n")
	for i := 0; i < 15; i++ {
		b.WriteString("\tfmt.Println(\"logout logic\")\n")
	}
	b.WriteString("}\n\n")
	b.WriteString("func ResetPassword(email string) error {\n")
	for i := 0; i < 15; i++ {
		b.WriteString("\tfmt.Println(\"reset logic\")\n")
	}
	b.WriteString("}\n")

	chunks := c.ChunkFile("auth.go", b.String())

	// Should have at least 3 function chunks.
	functionChunks := 0
	for _, ch := range chunks {
		if ch.FunctionName != "" {
			functionChunks++
		}
	}
	if functionChunks < 3 {
		t.Errorf("expected at least 3 function chunks, got %d (total chunks: %d)", functionChunks, len(chunks))
	}

	// Check that Login is classified as high risk (auth keyword).
	for _, ch := range chunks {
		if ch.FunctionName == "Login" && ch.RiskLevel != "high" {
			t.Errorf("Login should be classified as high risk, got %q", ch.RiskLevel)
		}
	}
}

func TestChunkFile_LargeFunction(t *testing.T) {
	c := NewChunker(20, 1) // Very small MaxLines to force splitting.

	var b strings.Builder
	b.WriteString("package main\n\n")
	b.WriteString("func bigFunction() {\n")
	for i := 0; i < 50; i++ {
		b.WriteString("\tx := 1\n")
	}
	b.WriteString("}\n")

	chunks := c.ChunkFile("big.go", b.String())
	if len(chunks) < 2 {
		t.Errorf("expected large function to be split into multiple chunks, got %d", len(chunks))
	}
}

func TestDetectLanguage(t *testing.T) {
	tests := map[string]string{
		"main.go":         "go",
		"app.py":          "python",
		"index.js":        "javascript",
		"server.ts":       "typescript",
		"App.tsx":         "typescript",
		"Main.java":       "java",
		"parser.c":        "c",
		"parser.cpp":      "cpp",
		"lib.rs":          "rust",
		"app.rb":          "ruby",
		"index.php":       "php",
		"unknown.xyz":     "unknown",
	}

	for file, expected := range tests {
		got := detectLanguage(file)
		if got != expected {
			t.Errorf("detectLanguage(%q) = %q, want %q", file, got, expected)
		}
	}
}

func TestClassifyRisk(t *testing.T) {
	tests := []struct {
		funcName string
		content  string
		expected string
	}{
		{"Login", "password check", "high"},
		{"handleAuth", "session token", "high"},
		{"parseInput", "decode data", "high"},
		{"executeQuery", "SELECT * FROM", "high"},
		{"formatOutput", "build response", "medium"},
		{"test_helper", "setup fixture", "low"},
	}

	for _, tt := range tests {
		got := classifyRisk(tt.funcName, tt.content)
		if got != tt.expected {
			t.Errorf("classifyRisk(%q, %q) = %q, want %q", tt.funcName, tt.content, got, tt.expected)
		}
	}
}

func TestBuildScannerPrompt(t *testing.T) {
	chunk := Chunk{
		ID:           "chunk_001",
		FilePath:     "auth/login.go",
		StartLine:    10,
		EndLine:      50,
		FunctionName: "Login",
		Content:      "func Login() {}",
		Context:      "package auth\nimport \"crypto\"",
		Language:     "go",
	}

	prompt := chunk.BuildScannerPrompt("Pattern 1: SQL injection...", 5)

	if !strings.Contains(prompt, "auth/login.go") {
		t.Error("prompt should contain file path")
	}
	if !strings.Contains(prompt, "Login") {
		t.Error("prompt should contain function name")
	}
	if !strings.Contains(prompt, "Pattern 1: SQL injection") {
		t.Error("prompt should contain knowledge injection")
	}
	if !strings.Contains(prompt, "chunk_001 of 5") {
		t.Error("prompt should contain chunk index and total")
	}
	if !strings.Contains(prompt, "lines 10-50") {
		t.Error("prompt should contain line range")
	}
}
