package health

import (
	typetostring "github.com/samber/go-type-to-string"
)

// DoServiceName returns the service name samber/do v2 infers for T, exactly
// as do.Provide[T], do.ProvideValue, and the other type-inferred registration
// functions name it. It is the string that appears as a key in the raw
// injector's health batch and therefore in [Response.Checks].
//
// Use it when a critical-service decision or a display-name table has to name
// a service that was registered by type:
//
//	probe := health.New(injector, health.WithCriticalService[*database.DB]())
//	label := health.DoServiceName[*database.DB]() // "*github.com/acme/app/db.DB"
//
// Both sides read the same naming helper samber/do uses, so the names agree
// by construction instead of by convention. Services registered with an
// explicit name (do.ProvideNamed, do.ProvideValue with a custom key) are not
// covered: pass their literal name to [WithCriticalServices].
func DoServiceName[T any]() string {
	return typetostring.GetType[T]()
}

// WithCriticalService marks the type-inferred samber/do service registered
// for T as critical, without spelling the container's internal service name
// by hand. It is the typed spelling of:
//
//	health.WithCriticalServices(health.DoServiceName[T]())
//
// and removes the two ways that hand-written name goes wrong: a typo (caught
// only at [Probe.Start] by [ErrUnknownCriticalService], and only if the
// service ran in the first batch) and a drift between the type a provider
// registers and the type string a critical list mentions.
//
//	probe := health.New(injector,
//	    health.WithCriticalService[*database.DB](),
//	    health.WithCriticalService[*storage.Backend](),
//	)
//
// The option appends to the critical set, so it composes with
// [WithCriticalServices] for explicitly named services. It only matches
// services registered under do's type-inferred name.
//
// It cannot work when a [HealthRecorder] renames check keys (mapping every
// do key to an operator display name is a common recorder shape): the
// type-inferred name never appears in the reported batch, so the typed
// helper marks a check that does not exist and Start fails with
// [ErrUnknownCriticalService] naming the raw do key. That recorder-renamed
// shape is the one place the typed spelling silently cannot work — pass the
// names the recorder actually reports to [WithCriticalServices] instead.
func WithCriticalService[T any]() Option {
	return WithCriticalServices(DoServiceName[T]())
}
