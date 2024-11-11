package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"sync"

	"github.com/dayvidpham/ipcache/internal/msgs"
)

var (
	prepInsertRegistrarStmt *sql.Stmt
)

const SQL_InsertRegistrar = `
	INSERT INTO
		Registrar (skid, unixTsUtc, ip)
	VALUES
		(?, ?, ?)
	ON CONFLICT(skid)
		DO UPDATE
		SET
			unixTsUtc = excluded.unixTsUtc,
			ip = excluded.ip
		WHERE 
			excluded.unixTsUtc > Registrar.unixTsUtc
	;`

func init() {
}

func initDb(ctx context.Context, db *sql.DB) (err error) {
	_, err = db.ExecContext(ctx,
		`CREATE TABLE IF NOT EXISTS
			Registrar(
				skid         TEXT NOT NULL COLLATE BINARY,
				unixTsUtc    INTEGER NOT NULL,
				ip           TEXT NOT NULL,
				PRIMARY KEY(skid)
			)
			WITHOUT ROWID
		;`)
	if err != nil {
		return err
	}

	// Check if Enum-style table exists, else create and insert values into it
	row := db.QueryRowContext(ctx,
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
		_, err = db.ExecContext(ctx,
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

		tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelLinearizable, ReadOnly: false})
		if err != nil {
			log.Println(err)
			return err
		}

		_, err = tx.ExecContext(ctx,
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

	_, err = db.ExecContext(ctx,
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

	prepInsertRegistrarStmt, err = db.PrepareContext(ctx, SQL_InsertRegistrar)
	if err != nil {
		return err
	}

	return
}

func getRegistrar(ctx context.Context, db *sql.DB) (registrar *sync.Map, err error) {
	var rows *sql.Rows
	if rows, err = db.Query(
		`SELECT
			*
		FROM
			Registrar
		;`,
	); err != nil {
		log.Println(err)
		return
	}
	defer rows.Close()

	registrar = &sync.Map{}
	var ignored int64
	for rows.Next() {
		var (
			clientId,
			clientStrIp string
			clientIP net.IP
		)

		err = rows.Scan(&clientId, &ignored, &clientStrIp)
		if err != nil {
			log.Println(err)
			return
		}

		clientIP = net.ParseIP(clientStrIp)
		if clientIP == nil {
			err = fmt.Errorf("[ERROR] Failed to parse `%s` as an IP", clientStrIp)
			return
		}
		log.Printf("[INFO] Got row from Registrar:\n\t- (skid: %s, ip: %s)\n\n", clientId, clientIP)
		registrar.Store(clientId, clientIP)
	}

	if err = rows.Err(); err != nil {
		log.Println(err)
		return
	}

	return
}

func insertRegistrar(
	ctx context.Context,
	db *sql.DB,
	client msgs.Client,
	ts int64,
) (err error) {
	tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelLinearizable, ReadOnly: false})
	if err != nil {
		return
	}

	log.Printf("[insertRegistrar] %+v\n", client)
	_, err = tx.
		StmtContext(ctx, prepInsertRegistrarStmt).
		ExecContext(ctx, client.Id, ts, client.IP.String())
	if err != nil {
		rollbackErr := tx.Rollback()
		return fmt.Errorf("%w\n\t%w", rollbackErr, err)
	}

	err = tx.Commit()
	return
}

// As a reference for boilerplate
func exampleQuery(ctx context.Context, db *sql.DB) (err error) {
	var rows *sql.Rows
	if rows, err = db.Query(
		`SELECT
			*
		FROM
			AuthorizationType;`); err != nil {
		log.Println(err)
		return err
	}

	defer rows.Close()
	for rows.Next() {
		var (
			c1 int
			c2 string
		)

		err = rows.Scan(&c1, &c2)
		if err != nil {
			log.Println(err)
			return err
		}
		log.Println(c1, c2)
	}

	if err = rows.Err(); err != nil {
		log.Println(err)
		return err
	}

	return
}
