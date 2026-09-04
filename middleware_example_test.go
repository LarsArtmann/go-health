package health_test

import (
	"fmt"
	"net/http"
	"net/http/httptest"

	"github.com/larsartmann/go-health"
	"github.com/samber/do/v2"
)

// bearerAuth is the spike's minimal middleware: it rejects requests without
// the expected bearer token before the probe handler runs. Middleware wraps
// plain http.HandlerFunc values — no library support needed.
func bearerAuth(token string, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer "+token {
			http.Error(w, "unauthorized", http.StatusUnauthorized)

			return
		}

		next(w, r)
	}
}

// ExampleProbe_ReadinessHandler_middleware demonstrates the middleware pattern for
// probe endpoints: handlers are ordinary http.HandlerFunc values, so
// net/http middleware composes by wrapping before registration. Protect
// readiness and startup with auth while keeping liveness open — kubelet
// cannot send credentials, and liveness must stay cheap and always-200.
func ExampleProbe_ReadinessHandler_middleware() {
	injector := do.New()

	do.ProvideNamed(injector, "database", func(_ do.Injector) (*exampleDB, error) {
		return &exampleDB{}, nil
	})
	_ = do.MustInvokeNamed[*exampleDB](injector, "database")

	probe := health.New(injector,
		health.WithCriticalServices("database"),
		health.WithRefreshInterval(0),
	)

	mux := http.NewServeMux()
	mux.HandleFunc("/healthz", probe.LivenessHandler())
	mux.HandleFunc("/readyz", bearerAuth("s3cret", probe.ReadinessHandler()))

	liveness := httptest.NewRecorder()
	mux.ServeHTTP(liveness, httptest.NewRequest(http.MethodGet, "/healthz", nil))

	noAuth := httptest.NewRecorder()
	mux.ServeHTTP(noAuth, httptest.NewRequest(http.MethodGet, "/readyz", nil))

	r := httptest.NewRequest(http.MethodGet, "/readyz", nil)
	r.Header.Set("Authorization", "Bearer s3cret")

	withAuth := httptest.NewRecorder()
	mux.ServeHTTP(withAuth, r)

	fmt.Println(liveness.Code, noAuth.Code, withAuth.Code)

	// Output:
	// 200 401 200
}
