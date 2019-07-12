package storage

import (
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/postgres"
	"github.com/spf13/viper"
	"github.com/teamnsrg/mida/jstrace"
	"github.com/teamnsrg/mida/log"
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
func CreatePostgresConnection(host string, port string, dbName string) (*gorm.DB, map[string]int, error) {

	log.Log.Info("Attempting connection:")
	log.Log.Infof("Host: [ %s ]", host)
	log.Log.Infof("Port: [ %s ]", port)
	log.Log.Infof("DB: [ %s ]", dbName)
	log.Log.Infof("Username: [ %s ]", viper.GetString("postgresuser"))
	log.Log.Infof("Password: [ %s ]", viper.GetString("postgrespass"))

	db, err := gorm.Open("postgres",
		"host="+host+
			" port="+port+
			" user="+viper.GetString("postgresuser")+
			" dbname="+"postgres"+
			" password="+viper.GetString("postgrespass"))
	if err != nil {
		return nil, nil, err
	}

	// Send the logs from gorm into our own logging infrastructure
	db.SetLogger(log.Log)

	// This will error if the database already exists. That's okay - we are going to connect to it anyway
	db = db.Exec("CREATE DATABASE " + dbName + " WITH TEMPLATE mida_template OWNER " + viper.GetString("postgresuser"))

	db, err = gorm.Open("postgres",
		"host="+host+
			" port="+port+
			" user="+viper.GetString("postgresuser")+
			" dbname="+dbName+
			" password="+viper.GetString("postgrespass"))
	if err != nil {
		return nil, nil, err
	}

	// Load the call name map from file
	var cn []CallNames
	db.Table("callnames").Find(&cn)
	callNameMap := make(map[string]int)
	for _, c := range cn {
		callNameMap[c.Type+" "+c.Class+"::"+c.Func] = c.CallId
	}

	return db, callNameMap, nil

}

func StoreJSTraceToDB(db *gorm.DB, callNameMap map[string]int, trace *jstrace.CleanedJSTrace) error {

	meta := Metadata{
		TS:       time.Now(),
		Url:      "test.test2",
		RandomId: "jklfadsjlksadg",
		Failed:   false,
	}

	tx := db.Begin()

	log.Log.Info(meta)
	tx.Create(&meta)

	for sId, scr := range trace.Scripts {
		scriptId, err := strconv.Atoi(sId)
		if err != nil {
			log.Log.Warningf("Skipping invalid script ID: [ %s ]", sId)
			continue
		}

		script := Script{
			CrawlId:  meta.CrawlId,
			ScriptId: scriptId,
			Length:   scr.Length,
			Calls:    len(scr.Calls),
			Url:      scr.Url,
			SHA1:     scr.SHA1,
		}

		tx.Create(&script)

		for i, call := range scr.Calls {
			if _, ok := callNameMap[call.T+" "+call.C+"::"+call.F]; ok {
				c := Call{
					CrawlID:  meta.CrawlId,
					ScriptID: scriptId,
					CallID:   callNameMap[call.T+" "+call.C+"::"+call.F],
					SeqNum:   i + 1,
				}
				tx.Create(&c)
			} else {
				log.Log.Warningf("Unknown API Call: %s %s::%s",
					call.T, call.C, call.F)
			}
		}
	}

	tx.Commit()

	return nil
}
