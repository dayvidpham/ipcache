package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"time"
)

func initDb(ctx context.Context, db *sql.DB) (err error) {
	_, err = db.Exec(
		`CREATE TABLE IF NOT EXISTS
			Registrar(
				skid    TEXT NOT NULL COLLATE BINARY,
				time    INTEGER NOT NULL,
				ip      TEXT NOT NULL,
				PRIMARY KEY(skid)
			)
			WITHOUT ROWID
		;`)
	if err != nil {
		return err
	}

	// Check if Enum-style table exists, else create and insert values into it
	row := db.QueryRow(
		`SELECT EXISTS(
				SELECT
					name
				FROM
					main.sqlite_schema
				WHERE
					name = 'AuthorizationType'
			)
		;`)

	var exists int
	err = row.Scan(&exists)
	if err != nil {
		log.Println(err)
		return err
	}

	// Create table
	if exists == 0 {
		_, err = db.Exec(
			`CREATE TABLE IF NOT EXISTS
					AuthorizationType(
						type INTEGER NOT NULL,
						desc TEXT NOT NULL,
						PRIMARY KEY(type ASC)
					)
			;`)
		if err != nil {
			return err
		}

		txTimeout := time.Second * 1
		txCtx, cancel := context.WithTimeout(ctx, txTimeout)
		defer cancel()

		tx, err := db.BeginTx(txCtx, &sql.TxOptions{Isolation: sql.LevelLinearizable, ReadOnly: false})
		if err != nil {
			log.Println(err)
			return err
		}

		_, err = tx.Exec(
			`INSERT INTO 
					'AuthorizationType' ('type', 'desc')
				VALUES
					(1, 'GetIP')
			;`)
		if err != nil {
			rollbackErr := tx.Rollback()
			return fmt.Errorf("%w\n\t%w", rollbackErr, err)
		}

		if err = tx.Commit(); err != nil {
			return err
		}
	}

	_, err = db.Exec(
		`CREATE TABLE IF NOT EXISTS
			AuthorizationGrants(
				owner   TEXT     NOT NULL COLLATE BINARY,
				other   TEXT     NOT NULL COLLATE BINARY,
				type    INTEGER  NOT NULL REFERENCES AuthorizationType(type),
				PRIMARY KEY(owner, other, type)
			)
			WITHOUT ROWID
		;`)
	if err != nil {
		return err
	}

	return
}
