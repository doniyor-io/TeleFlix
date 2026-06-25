package instagramwebhook

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestAuthorizedBearer(t *testing.T) {
	tests := []struct {
		name       string
		header     string
		secret     string
		authorized bool
	}{
		{
			name:       "missing token",
			secret:     "meta-secret",
			authorized: false,
		},
		{
			name:       "wrong scheme",
			header:     "Basic meta-secret",
			secret:     "meta-secret",
			authorized: false,
		},
		{
			name:       "wrong token",
			header:     "Bearer wrong",
			secret:     "meta-secret",
			authorized: false,
		},
		{
			name:       "valid token",
			header:     "Bearer meta-secret",
			secret:     "meta-secret",
			authorized: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodPost, "/webhook/instagram", nil)
			if tt.header != "" {
				req.Header.Set("Authorization", tt.header)
			}

			if got := authorizedBearer(req, tt.secret); got != tt.authorized {
				t.Fatalf("authorizedBearer() = %t, want %t", got, tt.authorized)
			}
		})
	}
}
