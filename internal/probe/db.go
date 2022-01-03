package probe

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v4"
)

var columnNames = []string{
	"time",
	"room",
	"temperature",
	"humidity",
	"battery",
}

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

func writeDBRow(ctx context.Context, t time.Time, room string, st SensorStatus, dburl, table string) error {
	conn, err := pgx.Connect(ctx, dburl)
	if err != nil {
		return err
	}
	defer conn.Close(ctx)

	columns := makeColumnString(columnNames)
	values := makeValuesString(columnNames)

	if _, err := conn.Exec(ctx,
		fmt.Sprintf("INSERT INTO %s(%s) VALUES(%s)", table, columns, values),
		t,
		room,
		st.Temperature,
		st.Humidity,
		st.Battery,
	); err != nil {
		return err
	}

	return nil
}
