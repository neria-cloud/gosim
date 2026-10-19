// Package hostabi exposes the selected native Go-family ABI to host tooling and
// the untranslated runtime engine. Generated application code does not import it.
//
// Coroutines and race primitives are family-owned. The shared Upcallcoro wrapper
// is verified on both pinned families: it owns only ordinary Go state and calls
// the selected Coro facade, with no private runtime layouts or linknames. Its
// race/nonrace variants preserve the scheduler's explicit upcall boundary.
package hostabi
