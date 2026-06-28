package main

import (
	"database/sql"
	"fmt"
	_ "github.com/mattn/go-sqlite3"
)

func main() {
	db, err := sql.Open("sqlite3", "E:\\open-source\\ai\\go-stock\\data\\stock.db")
	if err != nil {
		fmt.Println("Error opening DB:", err)
		return
	}
	defer db.Close()

	rows, err := db.Query("PRAGMA table_info(all_stock_info)")
	if err != nil {
		fmt.Println("Error querying schema:", err)
		return
	}
	defer rows.Close()

	fmt.Println("=== all_stock_info columns ===")
	for rows.Next() {
		var cid int
		var name, ctype string
		var notnull, pk int
		var dflt sql.NullString
		rows.Scan(&cid, &name, &ctype, &notnull, &dflt, &pk)
		fmt.Printf("  %s (%s)\n", name, ctype)
	}

	var count int
	db.QueryRow("SELECT COUNT(*) FROM all_stock_info").Scan(&count)
	fmt.Printf("\nTotal rows: %d\n", count)
	
	if count > 0 {
		rows2, _ := db.Query("SELECT secucode, securitynameabbr FROM all_stock_info LIMIT 5")
		defer rows2.Close()
		fmt.Println("\nSample data:")
		for rows2.Next() {
			var code, name string
			rows2.Scan(&code, &name)
			fmt.Printf("  %s - %s\n", code, name)
		}
	}
}
