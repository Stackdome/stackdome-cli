package output

import (
	"testing"

	openapi "github.com/Stackdome/stackdome/pkg/api/openapi"
)

func TestFormatPortsDefaultsProtocol(t *testing.T) {
	empty := ""
	tcp := "TCP"
	got := formatPorts([]openapi.Port{
		{Number: 80, Protocol: &empty},
		{Number: 443},
		{Number: 5432, Protocol: &tcp},
	})
	want := "80/HTTP, 443/HTTP, 5432/TCP"
	if got != want {
		t.Errorf("formatPorts = %q, want %q", got, want)
	}
}
