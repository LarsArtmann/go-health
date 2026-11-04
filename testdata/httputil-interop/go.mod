// Fixture module: proof that the composition recipes documented in
// docs/httputil-integration.md compile and hold against the published
// github.com/larsartmann/httputil release. It deliberately sits outside the
// root module so the library keeps its three-dependency policy; nothing here
// ships to consumers.
//
// Versioning convention (mirrors httputil's testdata/otelinterop fixture):
// httputil and server_timing resolve from the module proxy at their released
// tags, while go-health is replaced with the working tree via the `../..`
// replace below — the fixture proves HEAD's wire format and handler behavior
// against the dependency release the fleet actually pins. The CI step
// `httputil interop fixture` (in .github/workflows/ci.yml, via
// `nix run .#interop`) fails loudly when a go-health change breaks a
// documented recipe; keep this module and the doc in lockstep.
module github.com/larsartmann/go-health/testdata/httputil-interop

go 1.27

require (
	github.com/larsartmann/go-health v0.9.0
	github.com/larsartmann/httputil v1.8.0
	github.com/larsartmann/httputil/server_timing v1.0.3
)

require (
	github.com/justinas/nosurf v1.2.0 // indirect
	github.com/larsartmann/go-branded-id v0.7.1 // indirect
	github.com/larsartmann/go-error-family v0.12.0 // indirect
	github.com/larsartmann/go-etag/entitytag v0.6.1 // indirect
	github.com/larsartmann/go-etag/server v0.6.1 // indirect
	github.com/samber/do/v2 v2.1.0 // indirect
	github.com/samber/go-type-to-string v1.8.0 // indirect
	golang.org/x/time v0.16.0 // indirect
)

replace github.com/larsartmann/go-health => ../..
