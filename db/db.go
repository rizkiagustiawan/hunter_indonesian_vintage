package db

import (
	"database/sql"
	"log"
	"strings"

	_ "modernc.org/sqlite"
)

var DBPath = "data/listings.db"

type Listing struct {
	ID       string
	Title    string
	PriceIDR int
	City     string
	Keyword  string
	URL      string
	FoundAt  string
}

func InitDB() error {
	db, err := sql.Open("sqlite", DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	query := `
	CREATE TABLE IF NOT EXISTS listings (
		id          TEXT PRIMARY KEY,
		title       TEXT NOT NULL,
		price_idr   INTEGER NOT NULL,
		city        TEXT NOT NULL,
		keyword     TEXT NOT NULL,
		url         TEXT NOT NULL,
		found_at    TEXT NOT NULL
	)`
	_, err = db.Exec(query)
	if err != nil {
		return err
	}
	log.Println("Database initialized")
	return nil
}

func FilterNew(listings []Listing) ([]Listing, error) {
	if len(listings) == 0 {
		return []Listing{}, nil
	}

	db, err := sql.Open("sqlite", DBPath)
	if err != nil {
		return nil, err
	}
	defer db.Close()

	ids := make([]interface{}, len(listings))
	placeholders := make([]string, len(listings))
	for i, l := range listings {
		ids[i] = l.ID
		placeholders[i] = "?"
	}

	query := "SELECT id FROM listings WHERE id IN (" + strings.Join(placeholders, ",") + ")"
	rows, err := db.Query(query, ids...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	existingIDs := make(map[string]bool)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			existingIDs[id] = true
		}
	}

	var newListings []Listing
	for _, l := range listings {
		if !existingIDs[l.ID] {
			newListings = append(newListings, l)
		}
	}

	log.Printf("New listings: %d / %d", len(newListings), len(listings))
	return newListings, nil
}

func SaveListings(listings []Listing) error {
	if len(listings) == 0 {
		return nil
	}

	db, err := sql.Open("sqlite", DBPath)
	if err != nil {
		return err
	}
	defer db.Close()

	tx, err := db.Begin()
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(`
		INSERT OR IGNORE INTO listings (id, title, price_idr, city, keyword, url, found_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`)
	if err != nil {
		return err
	}
	defer stmt.Close()

	for _, l := range listings {
		_, err = stmt.Exec(l.ID, l.Title, l.PriceIDR, l.City, l.Keyword, l.URL, l.FoundAt)
		if err != nil {
			tx.Rollback()
			return err
		}
	}
	err = tx.Commit()
	if err != nil {
		return err
	}

	log.Printf("Saved %d listings to DB", len(listings))
	return nil
}
