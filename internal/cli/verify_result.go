package cli

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/dirloom/dirloom/internal/app"
	"github.com/dirloom/dirloom/internal/identity"
)

const verifyResultSchemaVersion = 1

type verifyJSONDocument struct {
	SchemaVersion       int               `json:"schemaVersion"`
	Status              string            `json:"status"`
	ExpectedFingerprint string            `json:"expectedFingerprint,omitempty"`
	ActualFingerprint   string            `json:"actualFingerprint,omitempty"`
	Diagnostic          *verifyDiagnostic `json:"diagnostic,omitempty"`
}

type verifyDiagnostic struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func writeVerifyText(writer io.Writer, result app.VerifyResult) error {
	switch result.Status {
	case app.VerifyMatch:
		_, err := io.WriteString(writer, "Structure matches snapshot.\n")
		return err
	case app.VerifyMismatch:
		_, err := fmt.Fprintf(writer, "Structure differs from snapshot.\nExpected: %s\nActual:   %s\n", result.ExpectedFingerprint.String(), result.ActualFingerprint.String())
		return err
	default:
		return fmt.Errorf("internal error: unsupported verify status %q", result.Status)
	}
}

func writeVerifyJSON(writer io.Writer, document verifyJSONDocument) error {
	encoder := json.NewEncoder(writer)
	encoder.SetEscapeHTML(false)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}

func verifySuccessDocument(result app.VerifyResult) verifyJSONDocument {
	return verifyJSONDocument{
		SchemaVersion:       verifyResultSchemaVersion,
		Status:              string(result.Status),
		ExpectedFingerprint: result.ExpectedFingerprint.String(),
		ActualFingerprint:   result.ActualFingerprint.String(),
	}
}

func verifyFailureDocument(kind app.VerifyFailureKind, err error, expected identity.Fingerprint) verifyJSONDocument {
	code, message := app.VerifyDiagnostic(err)
	document := verifyJSONDocument{
		SchemaVersion: verifyResultSchemaVersion,
		Status:        string(kind),
		Diagnostic: &verifyDiagnostic{
			Code:    code,
			Message: message,
		},
	}
	if kind == app.FailureObservation && expected.Version != 0 {
		document.ExpectedFingerprint = expected.String()
	}
	return document
}
