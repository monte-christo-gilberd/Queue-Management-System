package telemetry

import (
	"context"
	"database/sql"
	"database/sql/driver"

	"github.com/XSAM/otelsql"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// OpenSQL membuka *sql.DB. Bila tracing aktif, query dibungkus otelsql sehingga tiap query menjadi span anak dari
// request yang memanggilnya; bila tidak, sama dengan sql.Open (tanpa overhead).
//
// Privasi: otelsql hanya mencatat TEMPLAT query (semua query QMS berparameter $1, $2, ...), tidak pernah nilai
// parameternya. Span database hanya dibuat bila sudah ada span induk yang direkam, jadi pekerjaan latar (listener,
// worker) tidak menghasilkan trace yatim. Span Rows/Prepare/Connect dihilangkan agar trace tidak berisik.
func OpenSQL(
	driverName string,
	dsn string,
) (*sql.DB, error) {
	// tracing mati: pakai database/sql biasa, tanpa lapisan tambahan
	if !enabled {
		return sql.Open(driverName, dsn)
	}
	// tracing aktif: bungkus driver dengan otelsql
	return otelsql.Open(driverName, dsn,
		// penanda jenis database, dipakai Tempo untuk membedakan span database dari span HTTP
		otelsql.WithAttributes(attribute.String("db.system", "postgresql")),
		otelsql.WithSpanOptions(otelsql.SpanOptions{
			// operasi internal driver yang tidak berguna di trace dibuang agar waterfall tetap ringkas
			OmitConnResetSession: true,
			OmitConnPrepare:      true,
			OmitRows:             true,
			OmitConnectorConnect: true,
			// hanya query yang punya span induk valid dan direkam; tanpa induk, tidak ada span
			SpanFilter: func(ctx context.Context, _ otelsql.Method, _ string, _ []driver.NamedValue) bool {
				// span context induk dari ctx pemanggil query
				sc := trace.SpanContextFromContext(ctx)
				return sc.IsValid() && sc.IsSampled()
			},
		}),
	)
}
