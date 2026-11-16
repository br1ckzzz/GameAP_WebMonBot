package bot

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/bwmarrin/discordgo"
)

func TestValidateSetupURL(t *testing.T) {
	tests := []struct {
		name    string
		rawURL  string
		wantErr bool
	}{
		// Rejected schemes
		{name: "empty", rawURL: "", wantErr: true},
		{name: "file scheme", rawURL: "file:///etc/passwd", wantErr: true},
		{name: "ftp scheme", rawURL: "ftp://example.com/api", wantErr: true},
		{name: "javascript scheme", rawURL: "javascript:alert(1)", wantErr: true},
		{name: "missing scheme", rawURL: "panel.example.com/api", wantErr: true},

		// Rejected loopback
		{name: "loopback ipv4", rawURL: "http://127.0.0.1:8080/api", wantErr: true},
		{name: "loopback ipv4 alt", rawURL: "http://127.0.0.2/api", wantErr: true},
		{name: "loopback ipv6", rawURL: "http://[::1]:8080/api", wantErr: true},
		{name: "localhost", rawURL: "http://localhost:8080/api", wantErr: true},
		{name: "sub localhost", rawURL: "https://sub.localhost/api", wantErr: true},

		// Rejected link-local / cloud metadata
		{name: "metadata ipv4", rawURL: "http://169.254.169.254/latest/meta-data/", wantErr: true},
		{name: "link-local ipv6", rawURL: "http://[fe80::1]/api", wantErr: true},

		// Rejected RFC1918 private networks
		{name: "rfc1918 10.x", rawURL: "http://10.0.0.1/api", wantErr: true},
		{name: "rfc1918 172.16.x", rawURL: "http://172.16.0.1/api", wantErr: true},
		{name: "rfc1918 172.31.x", rawURL: "http://172.31.255.254/api", wantErr: true},
		{name: "rfc1918 192.168.x", rawURL: "http://192.168.1.100/api", wantErr: true},

		// Rejected unspecified
		{name: "unspecified ipv4", rawURL: "http://0.0.0.0:8080/api", wantErr: true},
		{name: "unspecified ipv6", rawURL: "http://[::]:8080/api", wantErr: true},

		// Valid public URLs
		{name: "valid https domain", rawURL: "https://panel.example.com/api/plugins/monitoring", wantErr: false},
		{name: "valid http public ip", rawURL: "http://93.184.216.34:8080/api", wantErr: false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateSetupURL(tc.rawURL)
			if tc.wantErr && err == nil {
				t.Errorf("validateSetupURL(%q) = nil, expected error", tc.rawURL)
			}
			if !tc.wantErr && err != nil {
				t.Errorf("validateSetupURL(%q) = %v, expected nil", tc.rawURL, err)
			}
		})
	}
}

func TestIsServerAdmin(t *testing.T) {
	h := &CommandHandler{}

	tests := []struct {
		name string
		i    *discordgo.InteractionCreate
		want bool
	}{
		{
			name: "nil member",
			i:    &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{}},
			want: false,
		},
		{
			name: "no permissions",
			i: &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
				Member: &discordgo.Member{Permissions: 0},
			}},
			want: false,
		},
		{
			name: "regular member",
			i: &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
				Member: &discordgo.Member{Permissions: discordgo.PermissionSendMessages | discordgo.PermissionViewChannel},
			}},
			want: false,
		},
		{
			name: "manage server",
			i: &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
				Member: &discordgo.Member{Permissions: discordgo.PermissionManageServer},
			}},
			want: true,
		},
		{
			name: "administrator",
			i: &discordgo.InteractionCreate{Interaction: &discordgo.Interaction{
				Member: &discordgo.Member{Permissions: discordgo.PermissionAdministrator},
			}},
			want: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := h.isServerAdmin(tc.i); got != tc.want {
				t.Errorf("isServerAdmin() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestSanitizeSetupError(t *testing.T) {
	tests := []struct {
		name         string
		err          error
		notContains  string
		wantContains string
	}{
		{
			name:         "http 500 with raw body",
			err:          errors.New("unexpected status HTTP 500: <html>secret internal stacktrace</html>"),
			notContains:  "secret internal stacktrace",
			wantContains: "HTTP 500",
		},
		{
			name:         "http 401 with token hint",
			err:          errors.New("unauthorized (HTTP 401): invalid or missing GameAP API Token ({\"debug\":\"secret_db_pass\"})"),
			notContains:  "secret_db_pass",
			wantContains: "401",
		},
		{
			name:         "http 403 with body",
			err:          errors.New("forbidden (HTTP 403): Bot API is disabled in plugin settings (internal_body)"),
			notContains:  "internal_body",
			wantContains: "403",
		},
		{
			name:         "timeout",
			err:          context.DeadlineExceeded,
			notContains:  "http://",
			wantContains: "таймаут",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := sanitizeSetupError(tc.err)
			if tc.notContains != "" && strings.Contains(got, tc.notContains) {
				t.Errorf("sanitizeSetupError leaked raw detail %q in %q", tc.notContains, got)
			}
			if tc.wantContains != "" && !strings.Contains(got, tc.wantContains) {
				t.Errorf("sanitizeSetupError(%v) = %q, expected to contain %q", tc.err, got, tc.wantContains)
			}
		})
	}
}
