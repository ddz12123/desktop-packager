package nginxproxy

import "testing"

func TestJoinPath(t *testing.T) {
	cases := []struct{ a, b, want string }{
		{"/v2", "users", "/v2/users"},
		{"/v2/", "users", "/v2/users"},
		{"/v2/", "/users", "/v2/users"},
		{"/v2", "/users", "/v2/users"},
		{"/", "users", "/users"},
		{"", "/api", "/api"},
		{"/api", "", "/api"},
	}
	for _, tc := range cases {
		if got := JoinPath(tc.a, tc.b); got != tc.want {
			t.Fatalf("JoinPath(%q, %q)=%q, want %q", tc.a, tc.b, got, tc.want)
		}
	}
}

func TestResolveUpstreamPath_NginxSemantics(t *testing.T) {
	cases := []struct {
		name        string
		location    string
		requestPath string
		targetPath  string
		rewrite     string
		want        string
	}{
		{
			name:        "proxy_pass with trailing slash strips location",
			location:    "/api/",
			requestPath: "/api/users",
			targetPath:  "/",
			want:        "/users",
		},
		{
			name:        "proxy_pass without URI keeps full path",
			location:    "/api/",
			requestPath: "/api/users",
			targetPath:  "",
			want:        "/api/users",
		},
		{
			name:        "proxy_pass with /v2/ prefix",
			location:    "/api/",
			requestPath: "/api/users",
			targetPath:  "/v2/",
			want:        "/v2/users",
		},
		{
			name:        "custom rewrite /v2",
			location:    "/api/",
			requestPath: "/api/users",
			targetPath:  "/",
			rewrite:     "/v2",
			want:        "/v2/users",
		},
		{
			name:        "custom rewrite /v2/",
			location:    "/api/",
			requestPath: "/api/users",
			targetPath:  "",
			rewrite:     "/v2/",
			want:        "/v2/users",
		},
		{
			name:        "location without trailing slash",
			location:    "/api",
			requestPath: "/api/users",
			targetPath:  "/",
			want:        "/users",
		},
		{
			name:        "exact location match with replace",
			location:    "/api/",
			requestPath: "/api",
			targetPath:  "/v2/",
			want:        "/v2/",
		},
		{
			name:        "nested path",
			location:    "/api/",
			requestPath: "/api/v1/users/1",
			targetPath:  "/",
			want:        "/v1/users/1",
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ResolveUpstreamPath(tc.location, tc.requestPath, tc.targetPath, tc.rewrite)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}

func TestMatchLocation(t *testing.T) {
	if !MatchLocation("/api/", "/api/users") {
		t.Fatal("expected match")
	}
	if !MatchLocation("/api/", "/api") {
		t.Fatal("expected /api to match /api/ location for convenience")
	}
	if MatchLocation("/api/", "/apiv2") {
		t.Fatal("should not match sibling prefix")
	}
}

func TestResolveUpstreamPath_NoURIKeepsNested(t *testing.T) {
	got := ResolveUpstreamPath("/api", "/api/v1/users", "", "")
	if got != "/api/v1/users" {
		t.Fatalf("got %q", got)
	}
}

func TestNormalizeLocation(t *testing.T) {
	if got := NormalizeLocation("api/"); got != "/api/" {
		t.Fatalf("got %q", got)
	}
	if got := NormalizeLocation("  /v1  "); got != "/v1" {
		t.Fatalf("got %q", got)
	}
}
