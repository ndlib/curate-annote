package annote

import (
	"database/sql"
	"log"
	"strings"
	"time"

	"github.com/BurntSushi/migration"
	_ "github.com/go-sql-driver/mysql"
)

// store pointer to sql database
type MysqlDB struct {
	db *sql.DB
}

var migrations = []migration.Migrator{
	migration1,
	migration2,
	migration3,
	migration4,
	migration5,
}

func migration1(tx migration.LimitedTx) error {
	var s = []string{
		`CREATE TABLE IF NOT EXISTS triples (
		id int PRIMARY KEY AUTO_INCREMENT,
		Subject varchar(64),
		Predicate varchar(255),
		Object text,
		INDEX i_subject (Subject),
		INDEX i_predicate (Predicate))`,
		`CREATE TABLE IF NOT EXISTS config (
		c_key varchar(255) PRIMARY KEY,
		c_value text)`,
	}
	return execlist(tx, s)
}

func migration2(tx migration.LimitedTx) error {
	var s = []string{
		`CREATE TABLE IF NOT EXISTS users (
		id int PRIMARY KEY AUTO_INCREMENT,
		username varchar(255),
		hashedpassword varchar(255),
		created datetime,
		orcid varchar(255),
		INDEX i_username (username),
		INDEX i_orcid (orcid))`,
	}
	return execlist(tx, s)
}

func migration3(tx migration.LimitedTx) error {
	var s = []string{
		`CREATE TABLE IF NOT EXISTS anno_uuid (
		id int PRIMARY KEY AUTO_INCREMENT,
		item varchar(64),
		username varchar(255),
		uuid varchar(64),
		status varchar(32),
		INDEX i_item (item),
		INDEX i_username (username),
		INDEX i_status (status))`,
	}
	return execlist(tx, s)
}

func migration4(tx migration.LimitedTx) error {
	var s = []string{
		`CREATE TABLE IF NOT EXISTS events (
			id int PRIMARY KEY AUTO_INCREMENT,
			event varchar(32),
			username varchar(255),
			timestamp datetime,
			other varchar(255),
			INDEX i_username (username),
			INDEX i_event (event),
			INDEX i_timestamp (timestamp))`,
	}
	return execlist(tx, s)
}

func migration5(tx migration.LimitedTx) error {
	var s = []string{
		`CREATE TABLE IF NOT EXISTS tots (
			id int PRIMARY KEY AUTO_INCREMENT,
			uuid varchar(64),
			canvas varchar(255),
			data text,
			creator  varchar(255),
			createdate datetime,
			modifiedby varchar(255),    
			modifydate datetime,
			INDEX i_uuid (uuid),
			INDEX i_canvas (canvas),
			INDEX i_creator (creator),
			INDEX i_modified (modifydate))`,
	}
	return execlist(tx, s)
}

// execlist exec's each item in the list, return if there is an error.
// Used to work around mysql driver not handling compound exec statements.
func execlist(tx migration.LimitedTx, stms []string) error {
	var err error
	for _, s := range stms {
		_, err = tx.Exec(s)
		if err != nil {
			break
		}
	}
	return err
}

type dbVersion struct {
	// SQL to get the version of this db, returns one row and one column
	GetSQL string
	// SQL to insert a new version of this db. takes one parameter, the new
	// version
	SetSQL string
	// the SQL to create the version table for this db
	CreateSQL string
}

func (d dbVersion) Get(tx migration.LimitedTx) (int, error) {
	v, err := d.get(tx)
	if err != nil {
		// we assume error means there is no migration table
		log.Println(err.Error())
		log.Println("Assuming this is because there is no migration table, yet")
		return 0, nil
	}
	return v, nil
}

func (d dbVersion) Set(tx migration.LimitedTx, version int) error {
	if err := d.set(tx, version); err != nil {
		if err := d.createTable(tx); err != nil {
			return err
		}
		return d.set(tx, version)
	}
	return nil
}

func (d dbVersion) get(tx migration.LimitedTx) (int, error) {
	var version int
	r := tx.QueryRow(d.GetSQL)
	if err := r.Scan(&version); err != nil {
		return 0, err
	}
	return version, nil
}

func (d dbVersion) set(tx migration.LimitedTx, version int) error {
	_, err := tx.Exec(d.SetSQL, version)
	return err
}

func (d dbVersion) createTable(tx migration.LimitedTx) error {
	_, err := tx.Exec(d.CreateSQL)
	if err == nil {
		err = d.set(tx, 0)
	}
	return err
}

var mysqlVersioning = dbVersion{
	GetSQL:    `SELECT max(version) FROM migration_version`,
	SetSQL:    `INSERT INTO migration_version (version, applied) VALUES (?, now())`,
	CreateSQL: `CREATE TABLE migration_version (version INTEGER, applied datetime)`,
}

// NewMySQL returns a Repository backed by a MySQL database, as determined
// by the connection string. An error is returned if any problems are run into.
func NewMySQL(conn string) (*MysqlDB, error) {
	conn += "?parseTime=true"
	db, err := migration.OpenWith(
		"mysql",
		conn,
		migrations,
		mysqlVersioning.Get,
		mysqlVersioning.Set,
	)
	if err != nil {
		return nil, err
	}
	return &MysqlDB{db: db}, nil
}

func (sq *MysqlDB) ReadConfig(key string) (string, error) {
	var v string
	row := sq.db.QueryRow(`SELECT c_value FROM config WHERE c_key = ? LIMIT 1`, key)
	err := row.Scan(&v)
	return v, err
}

func (sq *MysqlDB) SetConfig(key string, value string) error {
	_, err := sq.db.Exec(`INSERT INTO config (c_key, c_value) VALUES (?, ?)
		ON DUPLICATE KEY UPDATE c_value = ?`,
		key,
		value,
		value,
	)
	return err
}

func (sq *MysqlDB) IndexItem(item CurateItem) error {
	tx, err := sq.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback() // does nothing if tx has been committed

	_, err = tx.Exec(
		`DELETE FROM triples WHERE subject = ?`,
		item.PID,
	)
	if err != nil {
		return err
	}

	for _, t := range item.Properties {
		_, err = tx.Exec(
			`INSERT INTO triples (subject, predicate, object)
			VALUES (?, ?, ?)`,
			item.PID,
			t.Predicate,
			t.Object,
		)
		if err != nil {
			return err
		}
	}
	err = tx.Commit()
	return err
}

func readCurateItems(rows *sql.Rows) ([]CurateItem, error) {
	var err error
	var result []CurateItem
	current := &CurateItem{}
	for rows.Next() {
		var subject string
		var pair Pair
		err2 := rows.Scan(&subject, &pair.Predicate, &pair.Object)
		if err2 != nil {
			err = err2
			continue
		}
		if current.PID == "" {
			current.PID = subject
		} else if current.PID != subject {
			result = append(result, *current)
			current = &CurateItem{PID: subject}
		}
		current.Properties = append(current.Properties, pair)
	}
	if current.PID != "" {
		result = append(result, *current)
	}
	return result, err
}

// FindItem returns a single CurateItem record identified by PID.
func (sq *MysqlDB) FindItem(pid string) (CurateItem, error) {
	rows, err := sq.db.Query(`
		SELECT subject, predicate, object
		FROM triples
		WHERE subject = ?
		ORDER BY id`,
		pid)
	if err != nil {
		return CurateItem{}, err
	}
	defer rows.Close()
	items, err := readCurateItems(rows)
	if len(items) == 0 {
		return CurateItem{}, err
	}
	return items[0], err
}

func (sq *MysqlDB) FindItemFiles(pid string) ([]CurateItem, error) {
	var result []CurateItem
	rows, err := sq.db.Query(`
		SELECT subject, predicate, object
		FROM triples
		WHERE subject IN (
			SELECT subject
			FROM triples
			WHERE predicate = "isPartOf" and object = ?)
		ORDER BY subject, id`,
		pid,
	)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	return readCurateItems(rows)
}

func (sq *MysqlDB) FindCollectionMembers(pid string) ([]CurateItem, error) {
	var result []CurateItem
	rows, err := sq.db.Query(`
		SELECT subject, predicate, object
		FROM triples
		WHERE subject IN (
			SELECT subject
			FROM triples
			WHERE predicate = "isMemberOfCollection" and object = ?)
		ORDER BY subject, id`,
		pid,
	)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	return readCurateItems(rows)
}

// FindAllRange returns a list of every purl in the database.
func (sq *MysqlDB) FindAllRange(offset, count int) ([]CurateItem, error) {
	// The deployed database is mysql 5.7, and there is no support for WITH
	// statements or using LIMIT in IN subqueries. So we use an interesting
	// JOIN trick to limit the number of items.
	// reference: https://stackoverflow.com/questions/3984097
	log.Println("findallrange", offset, count)
	var result []CurateItem
	rows, err := sq.db.Query(`
		SELECT t.subject, t.predicate, t.object
		FROM triples AS t
		    LEFT JOIN (SELECT subject FROM triples
			WHERE predicate = "af-model" AND
			object NOT IN ("GenericFile", "Person", "Profile")
			LIMIT ? OFFSET ?) AS z
		    ON z.subject = t.subject
		WHERE z.subject IS NOT NULL
		ORDER BY t.subject, t.id`,
		count,
		offset,
	)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	return readCurateItems(rows)
}

// support the Searcher interface

func (sq *MysqlDB) Search(q SearchQuery) (SearchResults, error) {
	items, err := sq.FindAllRange(q.Start, q.NumRows)
	return SearchResults{Items: items}, err
}
func (sq *MysqlDB) IndexRecord(item CurateItem) {}
func (sq *MysqlDB) IndexBatch(source Batcher)   {}

// user things

func (sq *MysqlDB) FindUser(username string) (*User, error) {
	var u User
	row := sq.db.QueryRow(`SELECT id, username, hashedpassword, created, orcid FROM users WHERE username = ? LIMIT 1`, username)
	err := row.Scan(&u.ID, &u.Username, &u.HashedPassword, &u.Created, &u.ORCID)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (sq *MysqlDB) FindUserByToken(token string) (*User, error) {
	var u User
	row := sq.db.QueryRow(`SELECT id, username, hashedpassword, created, orcid FROM users WHERE hashedpassword = ? LIMIT 1`, token)
	err := row.Scan(&u.ID, &u.Username, &u.HashedPassword, &u.Created, &u.ORCID)
	if err != nil {
		return nil, err
	}
	return &u, nil
}

func (sq *MysqlDB) SaveUser(user *User) error {
	var err error
	if user.ID == 0 {
		_, err = sq.db.Exec(`INSERT INTO users (username, hashedpassword, created, orcid) VALUES (?, ?, ?, ?)`,
			user.Username,
			user.HashedPassword,
			user.Created,
			user.ORCID,
		)
	} else {
		_, err = sq.db.Exec(`UPDATE users SET username = ?, hashedpassword=?, created = ?, orcid =? WHERE id = ?`,
			user.Username,
			user.HashedPassword,
			user.Created,
			user.ORCID,
			user.ID,
		)
	}
	return err
}

func (sq *MysqlDB) SearchItemUUID(item string, username string, status string) ([]ItemUUID, error) {
	var clauses []string
	var params []interface{}

	if item != "" {
		clauses = append(clauses, "item = ?")
		params = append(params, item)
	}
	if username != "" {
		clauses = append(clauses, "username = ?")
		params = append(params, username)
	}
	if status != "" {
		clauses = append(clauses, "status = ?")
		params = append(params, status)
	}
	q := `SELECT item, username, uuid, status FROM anno_uuid`
	if len(clauses) > 0 {
		q = q + ` WHERE ` + strings.Join(clauses, " AND ")
	}
	var result []ItemUUID
	rows, err := sq.db.Query(q, params...)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		var current ItemUUID
		err2 := rows.Scan(&current.Item, &current.Username, &current.UUID, &current.Status)
		if err2 != nil {
			err = err2
			continue
		}
		result = append(result, current)
	}

	return result, err
}

func (sq *MysqlDB) UpdateUUID(record ItemUUID) error {
	// see if this is an insert or an update
	r, err := sq.SearchItemUUID(record.Item, record.Username, "")
	if err != nil {
		return err
	}
	if len(r) == 0 {
		// insert new record
		_, err = sq.db.Exec(`INSERT INTO anno_uuid (item, username, uuid, status) VALUES (?, ?, ?, ?)`,
			record.Item,
			record.Username,
			record.UUID,
			record.Status,
		)
		return err
	}
	// update existing record
	_, err = sq.db.Exec(`UPDATE anno_uuid SET uuid = ?, status = ? WHERE item = ? AND username = ?`,
		record.UUID,
		record.Status,
		record.Item,
		record.Username,
	)
	return err
}

func (sq *MysqlDB) RecordEvent(event string, user *User, other string) {
	t := time.Now()
	log.Println("EVENT", event, user.Username, other, t)
	_, err := sq.db.Exec(`INSERT INTO events (event, username, timestamp, other) VALUES (?, ?, ?, ?)`,
		event,
		user.Username,
		t,
		other,
	)
	if err != nil {
		log.Println(err)
	}
}

//
// Annatot functions
//

func (sq *MysqlDB) TotsByCanvas(canvas string) ([]tots, error) {
	log.Println("totbycanvas", canvas)
	var result []tots
	rows, err := sq.db.Query(`
		SELECT id, uuid, canvas, data, creator, createdate, modifiedby, modifydate
		FROM tots
		WHERE canvas = ?
		ORDER BY id`,
		canvas,
	)
	if err != nil {
		return result, err
	}
	defer rows.Close()
	for rows.Next() {
		t := tots{}
		err = rows.Scan(&t.ID,
			&t.UUID,
			&t.Canvas,
			&t.Data,
			&t.Creator,
			&t.CreateDate,
			&t.ModifiedBy,
			&t.ModifyDate,
		)
		result = append(result, t)
	}
	if rows.Err() != nil {
		err = rows.Err()
	}
	return result, err
}

func (sq *MysqlDB) TotCreate(tot tots) error {
	log.Println("TotCreate", tot)
	_, err := sq.db.Exec(`INSERT INTO tots (uuid, canvas, data, creator, createdate, modifiedby, modifydate)
	VALUES (?, ?, ?, ?, ?, ?, ?)`,
		tot.UUID,
		tot.Canvas,
		tot.Data,
		tot.Creator,
		tot.CreateDate,
		tot.ModifiedBy,
		tot.ModifyDate,
	)
	return err
}

// update the annotation given by tot.UUID to have tot.Data
func (sq *MysqlDB) TotUpdateData(tot tots) error {
	log.Println("TotUpdateData", tot)
	_, err := sq.db.Exec(`UPDATE tots SET data = ?, modifiedby = ?, modifydate = ? WHERE uuid = ?`,
		tot.Data,
		tot.ModifiedBy,
		tot.ModifyDate,
		tot.UUID,
	)
	return err

}
