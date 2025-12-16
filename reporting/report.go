
package reporting

import (
    "database/sql"
    "fmt"
)

// TotalsByCategory returns duration_seconds summed per category for local dates within [fromDate, toDate] inclusive.
// fromDate/toDate format: "YYYY-MM-DD"
type CategoryTotal struct {
    Category       string
    TotalSeconds   int64
    FormattedHuman string // optional formatting done by caller; we return raw seconds
}

func TotalsByCategory(db *sql.DB, fromDate, toDate string) ([]CategoryTotal, error) {
    rows, err := db.Query(`
SELECT category, SUM(duration_seconds) AS total_seconds
FROM interval_days
WHERE date_local >= ? AND date_local <= ?
GROUP BY category
ORDER BY total_seconds DESC;
`, fromDate, toDate)
    if err != nil {
        return nil, fmt.Errorf("query totals: %w", err)
    }
    defer rows.Close()

    var res []CategoryTotal
    for rows.Next() {
        var ct CategoryTotal
        if err := rows.Scan(&ct.Category, &ct.TotalSeconds); err != nil {
            return nil, err
        }
        res = append(res, ct)
    }
    return res, rows.Err()
}

// PresenceDays returns a sorted list of distinct local dates where any work occurred (duration_seconds > 0).
func PresenceDays(db *sql.DB, fromDate, toDate string) ([]string, error) {
    rows, err := db.Query(`
SELECT DISTINCT date_local
FROM interval_days
WHERE date_local >= ? AND date_local <= ? AND duration_seconds > 0
ORDER BY date_local;
`, fromDate, toDate)
    if err != nil {
        return nil, fmt.Errorf("query presence days: %w", err)
    }
    defer rows.Close()

    var days []string
    for rows.Next() {
        var d string
        if err := rows.Scan(&d); err != nil {
            return nil, err
        }
        days = append(days, d)
    }
    return    return days, rows.Err()


