package handlers

import (
	"mime"
	"strings"
	"testing"

	"inspection-app/internal/models"
)

func TestDocumentFileName_NumberAndAddress(t *testing.T) {
	insp := models.Inspection{ActNumber: "391-230726", Address: "г. Пермь, ул. Революции, 21А, кв. 148 (подъезд 2)"}
	utf8Name, asciiName := documentFileName(insp, "pdf")
	if utf8Name != "Акт 391-230726 - г. Пермь, ул. Революции, 21А, кв. 148 (подъезд 2).pdf" {
		t.Errorf("utf8 name = %q", utf8Name)
	}
	if asciiName != "act_391-230726.pdf" {
		t.Errorf("ascii name = %q", asciiName)
	}
}

func TestDocumentFileName_NoAddress_AndUnsafeChars(t *testing.T) {
	insp := models.Inspection{ActNumber: "15/2026", Address: "  "}
	utf8Name, asciiName := documentFileName(insp, "pdf")
	if utf8Name != "Акт 15 2026.pdf" {
		t.Errorf("utf8 name = %q", utf8Name)
	}
	if asciiName != "act_15_2026.pdf" {
		t.Errorf("ascii name = %q", asciiName)
	}
	insp = models.Inspection{ActNumber: "1", Address: "ул. \"Мира\"\\д. 3: кв?*<>|\n5"}
	utf8Name, _ = documentFileName(insp, "pdf")
	if strings.ContainsAny(utf8Name, `/\:*?"<>|`+"\n") {
		t.Errorf("unsafe chars left: %q", utf8Name)
	}
	if utf8Name != "Акт 1 - ул. Мира д. 3 кв 5.pdf" {
		t.Errorf("utf8 name = %q", utf8Name)
	}
}

func TestDocumentFileName_Truncated(t *testing.T) {
	insp := models.Inspection{ActNumber: "1", Address: strings.Repeat("очень длинный адрес ", 20)}
	utf8Name, _ := documentFileName(insp, "pdf")
	if n := len([]rune(utf8Name)); n > maxDocNameLen+4 {
		t.Errorf("name too long: %d runes", n)
	}
	if !strings.HasSuffix(utf8Name, ".pdf") {
		t.Errorf("suffix lost: %q", utf8Name)
	}
}

func TestContentDisposition_ParsesBackToUTF8(t *testing.T) {
	utf8Name, asciiName := documentFileName(models.Inspection{ActNumber: "7-010126", Address: "Пермь, Ленина 1; кв. 2"}, "pdf")
	header := contentDisposition(utf8Name, asciiName)
	disp, params, err := mime.ParseMediaType(header)
	if err != nil {
		t.Fatalf("header %q: %v", header, err)
	}
	if disp != "attachment" {
		t.Errorf("disposition = %q", disp)
	}
	if params["filename"] != utf8Name {
		t.Errorf("decoded filename = %q, want %q", params["filename"], utf8Name)
	}
	if strings.ContainsAny(header, "\r\n") {
		t.Error("header contains newline")
	}
}
