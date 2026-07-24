package html

import (
	"strings"
	"testing"
)

func TestConvertToText_BasicHTML(t *testing.T) {
	input := "<p>Hello, world!</p>"
	got, err := ConvertToText(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "Hello, world!") {
		t.Errorf("expected output to contain 'Hello, world!', got %q", got)
	}
}

func TestConvertToText_EmptyInput(t *testing.T) {
	_, err := ConvertToText("")
	if err == nil {
		t.Fatal("expected error for empty input, got nil")
	}
}

func TestConvertToText_LinkPreserved(t *testing.T) {
	input := `<a href="https://example.com">Click here</a>`
	got, err := ConvertToText(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "https://example.com") {
		t.Errorf("expected link URL to be preserved, got %q", got)
	}
}

func TestConvertToText_Formatting(t *testing.T) {
	input := "<ul><li>Item 1</li><li>Item 2</li></ul>"
	got, err := ConvertToText(input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !strings.Contains(got, "Item 1") || !strings.Contains(got, "Item 2") {
		t.Errorf("expected list items to be preserved, got %q", got)
	}
}
