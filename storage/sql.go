package storage

import (
	"database/sql"
	"github.com/lib/pq"
	"github.com/spf13/viper"
	"github.com/teamnsrg/mida/log"
	"github.com/teamnsrg/mida/types"
	"strconv"
	"time"
)

type CallNames struct {
	CallId int    `gorm:"not null"`
	Type   string `gorm:"not null"`
	Class  string `gorm:"not null"`
	Func   string `gorm:"not null"`
}

type Call struct {
	CrawlID  int `gorm:"not null"`
	ScriptID int `gorm:"not null"`
	CallID   int `gorm:"not null"`
	SeqNum   int `gorm:"not null"`
}

type Script struct {
	CrawlId  int    `gorm:"not null"`
	ScriptId int    `gorm:"not null"`
	Length   int    `gorm:"not null"`
	Calls    int    `gorm:"not null"`
	Url      string `gorm:"not null"`
	SHA1     string `gorm:"not null"`
}

type Metadata struct {
	CrawlId  int       `gorm:"autoincrement;not null;primary_key"`
	TS       time.Time `gorm:"not null"`
	Url      string    `gorm:"not null"`
	RandomId string    `gorm:"not null"`
	Failed   bool      `gorm:"not null"`
}

// CreatePostgresConnection connects to the postgres server and creates the specified database, if it does not
// already exist.
func CreatePostgresConnection(host string, port string, dbName string) (*sql.DB, map[string]int, error) {
	db, err := sql.Open("postgres",
		"host="+host+
			" port="+port+
			" user="+viper.GetString("postgresuser")+
			" dbname="+"postgres"+
			" password="+viper.GetString("postgrespass"))
	if err != nil {
		return nil, nil, err
	}

	// This will error if the database already exists. That's okay - we are going to connect to it anyway
	_, err = db.Exec("CREATE DATABASE " + dbName + " WITH TEMPLATE mida_template OWNER " + viper.GetString("postgresuser"))
	if err != nil && err.Error() != "pq: database \""+dbName+"\" already exists" {
		log.Log.Error(err)
	}

	db, err = sql.Open("postgres",
		"host="+host+
			" port="+port+
			" user="+viper.GetString("postgresuser")+
			" dbname="+dbName+
			" password="+viper.GetString("postgrespass"))
	if err != nil {
		return nil, nil, err
	}

	// Load the call name map from file
	rows, err := db.Query("SELECT call_id, type, class, func FROM callnames")
	if err != nil {
		return nil, nil, err
	}
	callNameMap := make(map[string]int)
	for rows.Next() {
		var callId int
		var callType, callClass, callFunc string
		err = rows.Scan(&callId, &callType, &callClass, &callFunc)

		callNameMap[callType+" "+callClass+"::"+callFunc] = callId
	}

	return db, callNameMap, nil

}

func StoreJSTraceToDB(db *sql.DB, callNameMap map[string]int, r *types.FinalMIDAResult) error {
	var crawlId int
	metaStmt := "INSERT INTO metadata (ts, url, random_id, failed) VALUES ($1, $2, $3, $4) RETURNING crawl_id"
	err := db.QueryRow(metaStmt, time.Now(), r.SanitizedTask.Url,
		r.SanitizedTask.RandomIdentifier, r.SanitizedTask.TaskFailed).Scan(&crawlId)
	if err != nil {
		log.Log.Fatal(err)
	}

	for sId, scr := range r.JSTrace.Scripts {
		scriptId, err := strconv.Atoi(sId)
		if err != nil {
			log.Log.Warningf("Skipping invalid script ID: [ %s ]", sId)
			continue
		}

		scriptStmt := `INSERT INTO scripts (crawl_id, script_id, url, length, calls, sha1) VALUES ($1, $2, $3, $4, $5, $6)`
		_, err = db.Exec(scriptStmt, crawlId, scriptId, scr.Url, scr.Length, len(scr.Calls), scr.SHA1)
		if err != nil {
			log.Log.Fatal(err)
		}

		tx, err := db.Begin()
		if err != nil {
			log.Log.Fatal(err)
		}

		callStmt, err := tx.Prepare(pq.CopyIn("calls", "crawl_id", "script_id", "call_id", "ret", "seq_num"))
		if err != nil {
			log.Log.Fatal(err)
		}

		for i, call := range scr.Calls {
			if callId, ok := callNameMap[call.T+" "+call.C+"::"+call.F]; ok {
				_, err = callStmt.Exec(crawlId, scriptId, callId, call.Ret.Val, i)
				if err != nil {
					log.Log.Fatal(err)
				}
			} else {
				log.Log.Warningf("Unknown API Call: %s %s::%s",
					call.T, call.C, call.F)
			}
		}

		_, err = callStmt.Exec()
		if err != nil {
			log.Log.Fatal(err)
		}

		err = callStmt.Close()
		if err != nil {
			log.Log.Fatal(err)
		}

		err = tx.Commit()
		if err != nil {
			log.Log.Fatal(err)
		}

	}

	return nil
}
