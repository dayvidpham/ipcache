package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/dayvidpham/ipcache/internal/msgs"
)

var (
	stmt_selectall         *sql.Stmt
	stmt_insert_registrar  *sql.Stmt
	stmt_insert_authgrants *sql.Stmt
	stmt_delete_authgrants *sql.Stmt
)

const SQL_SelectAll =
	`SELECT
		*
	FROM
		?
	;`


const SQL_CreateTable_Registrar = 
	`CREATE TABLE IF NOT EXISTS
		Registrar(
			skid
				TEXT
				NOT NULL
				COLLATE BINARY,
			unixTsUtc
				INTEGER
				NOT NULL,
			ip
				TEXT
				NOT NULL,
			PRIMARY KEY(skid)
		)
		WITHOUT ROWID
	;`
const SQL_InsertRow_Registrar = 
	`INSERT INTO
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
const SQL_SelectAll_Registrar = 
	`SELECT
		*
	FROM
		Registrar
	;`



const SQL_CreateTable_AuthType = `CREATE TABLE IF NOT EXISTS
	AuthorizationType(
		type 
			INTEGER
			NOT NULL,
		desc
			TEXT
			NOT NULL,
		PRIMARY KEY(type ASC)
	)
	;`
const SQL_TableExists_AuthType = 
	`SELECT EXISTS(
		SELECT
			name
		FROM
			main.sqlite_schema
		WHERE
			name = 'AuthorizationType'
	)
	;`
const SQL_InitTable_AuthType =
	`INSERT INTO 
		'AuthorizationType' ('type', 'desc')
	VALUES
		(0, 'GetIP')
	;`
const SQL_SelectAll_AuthGrants = 
	`SELECT
		*
	FROM
		AuthorizationType
	;`


const SQL_CreateTable_AuthGrants =
	`CREATE TABLE IF NOT EXISTS
		AuthorizationGrants(
			owner
				TEXT
				NOT NULL
				COLLATE BINARY,
			other
				TEXT
				NOT NULL
				COLLATE BINARY,
			type
				INTEGER
				NOT NULL,
			FOREIGN KEY(type)
				REFERENCES AuthorizationType(type)
				ON UPDATE RESTRICT
				ON DELETE RESTRICT,
			PRIMARY KEY(owner, other, type)
		)
		WITHOUT ROWID
	;`
const SQL_InsertRow_AuthGrants =
	`INSERT INTO 
		'AuthorizationGrants' ('owner', 'other', 'type')
	VALUES
		(?, ?, ?)
	;`
const SQL_DeleteRow_AuthGrants =
	`DELETE FROM
		'AuthorizationGrants'
	WHERE
		owner = ? AND
		other = ? AND
		type = ?
	;`


type IPCache struct {
	db *sql.DB
	timeout time.Duration

	registrar  *Registrar
	authGrants *AuthGrants
}

func (c *IPCache) Register(
	skid string,
	unixTsUtc int64,
	ip string,
) (err error) {
	// Do auth, perms check
	return
}
func (c *IPCache) GetIPs(self string) (err error) {
	// Do auth, perms check
	return
}
func (c *IPCache) GetIP(self string, other string) (err error) {
	// Do auth, perms check
	return
}
func (c *IPCache) GrantAuth(
	owner string,
	other string,
	atype AuthType,
) (err error) {
	return
}
func (c *IPCache) RevokeAuth(entry AuthGrantsRow) (err error) {
	return
}

func NewIPCache(
	ctx context.Context,
	db *sql.DB,
	timeout time.Duration,
) (c *IPCache, err error) {
	err = initDb(ctx, db)
	if err != nil {
		return
	}

	c = &IPCache{
		db: db,
		timeout: timeout,
	}

	return
}

type Registrar struct {
	m *sync.Map
	t *RegistrarTable
}

type RegistrarTable struct {
	selectAll *sql.Stmt
	selectRow *sql.Stmt
	insert    *sql.Stmt
}

type RegistrarRow struct {
	Skid string
	UnixTsUtc int64
	IP net.IP
}

func NewRegistrar() {

}

func (r *RegistrarTable) SelectAll(ctx context.Context) (rrows []RegistrarRow, err error) {
	rows, err := r.selectAll.QueryContext(ctx)
	if err != nil {
		return
	}
	defer rows.Close()

	return
}

type AuthType int64
const (
	AuthT_GetIP AuthType = 0
)

type AuthGrants struct {
	m *sync.Map
	t *AuthGrantsTable
}
type AuthGrantsTable struct {
	selectAll *sql.Stmt
	selectRow *sql.Stmt
	insert    *sql.Stmt
	remove    *sql.Stmt
}

type AuthGrantsRow struct {
	Owner string
	Other string
	Type AuthType
}


func initDb(ctx context.Context, db *sql.DB) (err error) {
	_, err = db.ExecContext(ctx, SQL_CreateTable_Registrar)
	if err != nil {
		return err
	}

	// Check if Enum-style table exists, else create and insert values into it
	row := db.QueryRowContext(ctx, SQL_TableExists_AuthType)

	var exists int
	err = row.Scan(&exists)
	if err != nil {
		log.Println(err)
		return err
	}

	// Create table
	if exists == 0 {
		_, err = db.ExecContext(ctx, SQL_CreateTable_AuthType)
		if err != nil {
			return err
		}

		tx, err := db.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelLinearizable, ReadOnly: false})
		if err != nil {
			log.Println(err)
			return err
		}

		_, err = tx.ExecContext(ctx, SQL_InitTable_AuthType)
		if err != nil {
			rollbackErr := tx.Rollback()
			return fmt.Errorf("%w\n\t%w", rollbackErr, err)
		}

		if err = tx.Commit(); err != nil {
			return err
		}
	}

	_, err = db.ExecContext(ctx, SQL_CreateTable_AuthGrants)
	if err != nil {
		return err
	}

	stmt_insert_registrar, err = db.PrepareContext(ctx, SQL_InsertRow_Registrar)
	if err != nil {
		return err
	}

	return
}

func getRegistrar(ctx context.Context, db *sql.DB) (registrar *sync.Map, err error) {
	var rows *sql.Rows
	if rows, err = db.QueryContext(ctx, SQL_SelectAll_Registrar); err != nil {
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
		StmtContext(ctx, stmt_insert_registrar).
		ExecContext(ctx, client.Id, ts, client.IP.String())
	if err != nil {
		rollbackErr := tx.Rollback()
		return fmt.Errorf("%w\n\t%w", rollbackErr, err)
	}

	err = tx.Commit()
	return
}

/*
 As a reference for boilerplate

func exampleQuery(ctx context.Context, db *sql.DB) (err error) {
	var rows *sql.Rows
	if rows, err = db.QueryContext(ctx, 
		`SELECT
			*
		FROM
			AuthorizationGrants
		;`); err != nil {
		log.Println(err)
		return err
	}
	if err != nil {
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
*/
