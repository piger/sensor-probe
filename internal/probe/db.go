package probe

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
	"github.com/piger/sensor-probe/internal/sensors/mijia"
)

var columnNames = []string{
	"time",
	"room",
	"temperature",
	"humidity",
	"battery",
}

var dbConnTimeout = 1 * time.Minute

func makeColumnString(names []string) string {
	return strings.Join(names, ",")
}

func makeValuesString(names []string) string {
	result := make([]string, len(names))
	for i := range names {
		result[i] = fmt.Sprintf("$%d", i+1)
	}

	return strings.Join(result, ",")
}

func writeDBRow(ctx context.Context, t time.Time, room string, data *mijia.Data, dburl, table string) error {
	ctx, cancel := context.WithTimeout(ctx, dbConnTimeout)
	defer cancel()

	conn, err := pgx.Connect(ctx, dburl)
	if err != nil {
		return fmt.Errorf("error connecting to DB: %w", err)
	}
	defer conn.Close(ctx)

	columns := makeColumnString(columnNames)
	values := makeValuesString(columnNames)

	if _, err := conn.Exec(ctx,
		fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", table, columns, values),
		t,
		room,
		data.Temperature,
		data.Humidity,
		data.Battery,
	); err != nil {
		return fmt.Errorf("error writing row to DB: %w", err)
	}

	return nil
}
