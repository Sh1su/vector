package kernel

import "context"

type dryRunKey struct{}

// WithDryRun markiert einen Kontext als Probelauf: Die Transaktion wird nach
// erfolgreicher Prüfung zurückgerollt (Vorschläge des Assistenten, ADR-026).
func WithDryRun(ctx context.Context) context.Context {
	return context.WithValue(ctx, dryRunKey{}, true)
}

// IsDryRun meldet, ob der Kontext ein Probelauf ist.
func IsDryRun(ctx context.Context) bool { v, _ := ctx.Value(dryRunKey{}).(bool); return v }
